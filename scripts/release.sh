#!/usr/bin/env bash
# Publish the three released artifacts: the container images, the Go SDK and the TypeScript SDK.
#
# ONE code path for local and CI. .github/workflows/release.yml calls exactly these subcommands, so a
# release that goes wrong in CI can be reproduced on a laptop with --dry-run, and nothing is published
# by a mechanism that only exists inside a YAML file.
#
# EACH ARTIFACT CARRIES ITS OWN VERSION and moves on its own tag:
#
#   v1.2.3              -> the three images       (scripts/release.sh images  1.2.3)
#   clients/go/v1.2.3   -> the Go SDK module      (scripts/release.sh go-sdk  1.2.3)
#   ts/v1.2.3           -> the npm package        (scripts/release.sh ts-sdk  1.2.3)
#
# The SDKs are versioned independently BECAUSE they are contract-derived: an SDK release that changes
# nothing is noise for every consumer who then has to read a diff to discover it was empty. So both
# SDK subcommands REFUSE to publish when the artifact is byte-identical to its last release, and
# refuse to publish a changed artifact under a version that is not greater. The rule is enforced here
# rather than remembered.
#
# Both SDKs are additionally checked against api/*.conjure.yml before publishing: publishing an SDK
# that no longer matches the contract it claims to be generated from is the one failure that would
# silently ship wrong types to every consumer.
#
# Usage:
#   scripts/release.sh check                  what each artifact would do right now, publishing nothing
#   scripts/release.sh images  <version>      build (and with --push, push) the three images
#   scripts/release.sh go-sdk  <version>      verify + tag the nested Go module
#   scripts/release.sh ts-sdk  <version>      verify + publish oikumenea-client to npm
#
# Flags:
#   --dry-run          do everything except push/publish/tag; prints exactly what would be run
#   --push             actually push images / tags / packages (default: build and verify only)
#   --platforms=LIST   override the image platforms (default: linux/amd64,linux/arm64)
#   --allow-dirty      permit a dirty working tree (refused by default — an artifact must be
#                      reproducible from a commit, and "it worked on my machine" starts here)
#
# Credentials (CI provides these; locally you need to be logged in already):
#   GHCR         docker login ghcr.io      — GITHUB_TOKEN in CI
#   Docker Hub   docker login docker.io    — DOCKERHUB_USERNAME / DOCKERHUB_TOKEN in CI
#   npm          npm login                 — NPM_TOKEN in CI
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

# ── configuration ───────────────────────────────────────────────────────────────────────────────
NAMESPACE="${RELEASE_NAMESPACE:-olehmushka}"
# Each entry is `host` (using NAMESPACE) or `host/namespace` — because the namespaces genuinely
# DIFFER: GHCR's is forced to the GitHub owner, while the Docker Hub account is whatever you signed up
# as, and assuming one from the other is how a push ends in `insufficient_scope: authorization failed`
# (authenticated fine, just not for that namespace).
#
# Narrowing with --registries also matters because one buildx invocation carries every tag, so an
# unauthenticated registry fails the whole build INCLUDING the half that would have worked.
RELEASE_REGISTRIES="${RELEASE_REGISTRIES:-ghcr.io/olehmushka,docker.io/olegamysk}"
PLATFORMS="${RELEASE_PLATFORMS:-linux/amd64,linux/arm64}"
NPM_PACKAGE="oikumenea-client"

# image name -> Dockerfile. The build context is the REPO ROOT for all three: the console consumes the
# TypeScript SDK as a `file:` dependency at clients/typescript, which lives outside web/.
IMAGE_NAMES=("oikumenea" "hermenea" "oikumenea-console")
IMAGE_FILES=("Dockerfile" "Dockerfile.hermenea" "web/Dockerfile")

DRY_RUN=0
PUSH=0
ALLOW_DIRTY=0

# ── plumbing ────────────────────────────────────────────────────────────────────────────────────
say()  { printf 'release: %s\n' "$*" >&2; }
warn() { printf 'release: WARNING: %s\n' "$*" >&2; }
die()  { printf 'release: ERROR: %s\n' "$*" >&2; exit 1; }

# run executes a command, or prints it under --dry-run. Every push/publish/tag goes through this, so
# --dry-run is a property of the script rather than a flag each call site remembers to honour.
run() {
  if [[ $DRY_RUN -eq 1 ]]; then
    printf 'release: [dry-run] %s\n' "$*" >&2
    return 0
  fi
  "$@"
}

# semver_gt A B — true when A is strictly greater than B. Pure sort -V, so it needs no python.
semver_gt() {
  [[ "$1" != "$2" ]] && [[ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" == "$1" ]]
}

require_semver() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
    || die "version '$1' is not semver (X.Y.Z, optionally -prerelease). Pass it WITHOUT the leading v."
}

require_clean_tree() {
  [[ $ALLOW_DIRTY -eq 1 ]] && { warn "--allow-dirty: publishing from a dirty tree"; return 0; }
  git diff --quiet && git diff --cached --quiet \
    || die "the working tree is dirty. An artifact must be reproducible from a commit — commit, stash, or pass --allow-dirty."
}

# last_tag_matching PREFIX — the highest semver tag with that prefix, or empty when none exists.
# The `|| true` is load-bearing: grep exits 1 on no match, and under `set -e` inside a command
# substitution that aborts the caller — so "no releases yet" would look like a crash, which is
# exactly the state this repo is in before its first release.
# EXCLUDE is the version being released: on the tag-triggered path the new tag already exists, so
# without this the guard would compare HEAD against ITSELF, conclude "identical" and refuse every
# CI release — the failure mode is a green local run and a red pipeline.
# Only X.Y.Z tags count as "the last release": a prerelease (1.2.3-rc1) is compared against the last
# STABLE one, which is the useful comparison — an rc that changed nothing since the previous rc is
# still worth cutting, an rc that changed nothing since the last release is not.
last_tag_matching() {
  local prefix="$1" exclude="${2:-}"
  git tag --list "$prefix*" \
    | sed "s|^$prefix||" \
    | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' \
    | { [[ -n "$exclude" ]] && grep -vxF "$exclude" || cat; } \
    | sort -V | tail -1 || true
}

# tree_changed_since TAG PATH… — is any of PATH different between TAG and HEAD? Compares the TREE, not
# the commit log: a path touched and reverted has not changed, and that is the honest answer to "does
# this artifact differ from the one already published".
tree_changed_since() {
  local tag="$1"; shift
  ! git diff --quiet "$tag" -- "$@"
}

# ensure_tag TAG MESSAGE — create the tag, or accept it when it already points HERE.
#
# Idempotence is what lets ONE code path serve both a laptop and a tag-triggered CI job: by the time
# the workflow runs, the tag it was triggered by exists and points at the commit being built. Dying
# on that would mean CI could not call the same script a human calls, which is the whole premise.
# A tag that exists and points SOMEWHERE ELSE is still fatal — that is a version being reused.
ensure_tag() {
  local tag="$1" msg="$2"
  if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    local at; at="$(git rev-list -n1 "$tag")"
    [[ "$at" == "$(git rev-parse HEAD)" ]] \
      || die "tag $tag already exists and points at ${at:0:12}, not HEAD — that version is taken."
    say "tag $tag already exists at HEAD (this is the tag-triggered path)"
    return 0
  fi
  run git tag -a "$tag" -m "$msg"
}

# docker_logged_in HOST — best-effort: is there a stored credential for this registry? Docker Hub is
# keyed by its legacy index URL rather than by hostname, which is why this is a lookup rather than a
# string compare. A credential HELPER stores nothing in the file, so its presence counts as
# "assume authenticated" — the push itself stays the real test; this only catches the common case
# early, which is the case that was hit.
docker_logged_in() {
  local host="$1" cfg="${DOCKER_CONFIG:-$HOME/.docker}/config.json"
  [[ -f "$cfg" ]] || return 1
  local key="$host"
  [[ "$host" == "docker.io" ]] && key="index.docker.io"
  grep -q "\"[^\"]*${key}" "$cfg" && return 0
  grep -q '"credsStore"' "$cfg" && return 0
  return 1
}

# ── the contract gate ───────────────────────────────────────────────────────────────────────────
# Both SDKs are GENERATED from api/*.conjure.yml (D-ClientSDK / D-Conjure). Publishing one whose
# committed sources no longer match the contract ships wrong types to every consumer, and it is
# invisible: the package builds, type-checks and imports cleanly — it is simply describing an API the
# server does not serve. The generators both have a --verify mode for exactly this.
verify_generated_matches_contract() {
  local which="$1"
  case "$which" in
    go)
      say "checking the Go mirrors against api/*.conjure.yml…"
      scripts/gen-action-params.sh --verify >/dev/null \
        || die "the IR-derived Go mirrors are STALE — run scripts/gen-action-params.sh and commit before releasing."
      ;;
    ts)
      say "checking clients/typescript/src/generated against api/*.conjure.yml…"
      scripts/gen-ts-client.sh --verify >/dev/null \
        || die "the generated TypeScript SDK is STALE — run 'make sdk' and commit before releasing."
      ;;
  esac
}

# ── images ──────────────────────────────────────────────────────────────────────────────────────
cmd_images() {
  local version="$1"
  require_semver "$version"
  require_clean_tree

  local REGISTRIES=()
  local host
  for host in ${RELEASE_REGISTRIES//,/ }; do
    # `host` alone takes the default namespace; `host/ns` carries its own.
    if [[ "$host" == */* ]]; then REGISTRIES+=("$host"); else REGISTRIES+=("${host}/${NAMESPACE}"); fi
  done
  [[ ${#REGISTRIES[@]} -gt 0 ]] || die "no registries selected (--registries=ghcr.io,docker.io)"

  local revision; revision="$(git rev-parse HEAD)"
  local created;  created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  local minor="${version%.*}"

  command -v docker >/dev/null || die "docker is not on PATH"
  docker buildx version >/dev/null 2>&1 \
    || die "docker buildx is required for the multi-platform build (platforms: $PLATFORMS)"

  # PREFLIGHT: can this builder actually produce every requested platform?
  #
  # The two Go images cross-compile (their build stage is pinned to $BUILDPLATFORM), so they need no
  # emulation at all. The CONSOLE does — its `npm run build` runs in a target-arch container — and
  # without binfmt registered that surfaces 200 lines into a build as `exec /bin/sh: exec format
  # error`, which reads like a broken Dockerfile rather than a missing host feature. CI installs the
  # handlers via docker/setup-qemu-action; a laptop usually has not.
  local supported; supported="$(docker buildx inspect --bootstrap 2>/dev/null | sed -n 's/^Platforms:[[:space:]]*//p')"
  local want
  for want in ${PLATFORMS//,/ }; do
    if [[ -n "$supported" && ",${supported// /}," != *",${want},"* ]]; then
      die "this builder cannot produce ${want} (it offers: ${supported:-unknown}).
       Register the emulators once with:  docker run --privileged --rm tonistiigi/binfmt --install all
       …or build a single platform:       scripts/release.sh images <version> --platforms=linux/amd64"
    fi
  done

  # CREDENTIAL PREFLIGHT. buildx pushes at EXPORT time, i.e. after every layer is built — so a missing
  # login surfaces as `failed to fetch anonymous token … 403` ten minutes into a release, with the word
  # "anonymous" as the only clue that nothing was ever sent.
  if [[ $PUSH -eq 1 ]]; then
    local reg rhost
    for reg in "${REGISTRIES[@]}"; do
      rhost="${reg%%/*}"
      if ! docker_logged_in "$rhost"; then
        die "not logged in to ${rhost} (pushing to ${reg}) — the push would fail at export, AFTER the whole build.
       ghcr.io:    echo <PAT-with-write:packages> | docker login ghcr.io -u ${NAMESPACE} --password-stdin
       docker.io:  docker login docker.io
       …or push only where you ARE authenticated:  --registries=<host>
       …or let CI do it: push a v* tag (GHCR uses the built-in GITHUB_TOKEN there)."
      fi
    done
  fi

  # A multi-platform build cannot load into the local daemon — the docker image store holds one
  # platform per tag. So without --push it is built and thrown away, which still proves every
  # Dockerfile compiles for every platform, and that is the point of a dry run.
  local output_flag="--output=type=cacheonly"
  [[ $PUSH -eq 1 ]] && output_flag="--push"

  say "building ${#IMAGE_NAMES[@]} images at $version for $PLATFORMS (push=$PUSH)"

  local i
  for i in "${!IMAGE_NAMES[@]}"; do
    local name="${IMAGE_NAMES[$i]}"
    local file="${IMAGE_FILES[$i]}"
    local tag_args=()
    local reg
    for reg in "${REGISTRIES[@]}"; do
      # Three moving tags per registry: the exact version, the minor line an operator can track for
      # patches, and latest. A digest is what a deployment should actually pin.
      tag_args+=(--tag "${reg}/${name}:${version}")
      tag_args+=(--tag "${reg}/${name}:${minor}")
      tag_args+=(--tag "${reg}/${name}:latest")
    done

    say "  ${name}  (${file})"
    run docker buildx build \
      --file "$file" \
      --platform "$PLATFORMS" \
      "${tag_args[@]}" \
      --label "org.opencontainers.image.title=${name}" \
      --label "org.opencontainers.image.version=${version}" \
      --label "org.opencontainers.image.revision=${revision}" \
      --label "org.opencontainers.image.created=${created}" \
      --label "org.opencontainers.image.source=https://github.com/${NAMESPACE}/go-oikumenea" \
      --label "org.opencontainers.image.licenses=Apache-2.0" \
      "$output_flag" \
      "$ROOT"
  done

  if [[ $PUSH -eq 1 ]]; then
    say "pushed:"
    for i in "${!IMAGE_NAMES[@]}"; do
      for reg in "${REGISTRIES[@]}"; do
        say "  ${reg}/${IMAGE_NAMES[$i]}:${version}"
      done
    done
  fi
}

# require_module_path_resolves — a Go module release is a TAG, and the proxy fetches it by the module
# path in go.mod, NOT by the repo you tagged. If the two disagree the tag publishes cleanly and
# `go get` 404s, with nothing to roll back: a version number is spent and the module is uninstallable.
#
# That is the live case here. go.mod says github.com/olegamysk/go-oikumenea while the repo is at
# github.com/olehmushka/go-oikumenea, so the declared path does not exist. Checked at RELEASE time
# rather than in CI, because it is exactly the moment the mistake becomes permanent.
require_module_path_resolves() {
  local modpath; modpath="$(sed -n 's/^module //p' clients/go/go.mod | head -1)"
  [[ -n "$modpath" ]] || die "cannot read the module path from clients/go/go.mod"
  say "module path: $modpath"

  # Only github.com paths can be checked this cheaply; anything else (a vanity domain) is the
  # operator's to verify.
  [[ "$modpath" == github.com/* ]] || { warn "non-github module path — not verified"; return 0; }

  local repo; repo="$(cut -d/ -f1-3 <<<"$modpath")"
  local code; code="$(curl -s -o /dev/null -w '%{http_code}' "https://$repo" 2>/dev/null || echo 000)"
  if [[ "$code" == "404" ]]; then
    die "the module path $modpath does not resolve (HTTP 404), but this repo is $(git remote get-url origin).
       A Go module is fetched by its go.mod path, so tagging this would publish a module that
       'go get $modpath/...' cannot install — and the version number would be spent.
       Fix the module path (go.mod + clients/go/go.mod + every import) to match where the repo lives,
       or make that path resolve, before releasing the Go SDK."
  fi
  [[ "$code" == "200" ]] || warn "could not verify $repo (HTTP $code) — check it before relying on the tag"
}

# ── the Go SDK ──────────────────────────────────────────────────────────────────────────────────
# A nested module (clients/go/go.mod), so "publishing" is a TAG — the module proxy serves it from
# there. Nothing is uploaded, which is why the guard below is the whole substance of this command.
cmd_go_sdk() {
  local version="$1"
  require_semver "$version"
  require_clean_tree

  local prefix="clients/go/v"
  local last; last="$(last_tag_matching "$prefix" "$version")"

  if [[ -n "$last" ]]; then
    if ! tree_changed_since "${prefix}${last}" clients/go; then
      die "clients/go is byte-identical to ${prefix}${last} — nothing to release. The SDK is contract-derived, so an empty version bump only costs every consumer a diff to read."
    fi
    semver_gt "$version" "$last" \
      || die "$version is not greater than the last released $last (${prefix}${last})."
    say "changed since ${prefix}${last}:"
    git diff --stat "${prefix}${last}" -- clients/go | tail -5 >&2
  else
    say "no previous ${prefix}* tag — this is the FIRST Go SDK release."
  fi

  require_module_path_resolves
  verify_generated_matches_contract go

  say "building + testing the nested module…"
  ( cd clients/go && go build ./... && go vet ./... && go test ./... ) \
    || die "the Go SDK module does not build/test clean — refusing to tag it."

  local tag="${prefix}${version}"
  ensure_tag "$tag" "Go client SDK $version"
  if [[ $PUSH -eq 1 ]]; then
    # A tag that is already on the remote is the CI case, not an error.
    if git ls-remote --exit-code --tags origin "$tag" >/dev/null 2>&1; then
      say "$tag is already on origin — nothing to push."
    else
      run git push origin "$tag"
    fi
    say "released: go get $(sed -n 's/^module //p' clients/go/go.mod | head -1)@v${version}"
  else
    say "tag $tag ready locally; re-run with --push to publish it."
  fi
}

# ── the TypeScript SDK ──────────────────────────────────────────────────────────────────────────
cmd_ts_sdk() {
  local version="$1"
  require_semver "$version"
  require_clean_tree

  local pkg_dir="clients/typescript"
  local prefix="ts/v"
  local last; last="$(last_tag_matching "$prefix" "$version")"

  if [[ -n "$last" ]]; then
    if ! tree_changed_since "${prefix}${last}" "$pkg_dir/src" "$pkg_dir/tsconfig.json" "$pkg_dir/README.md"; then
      # Only package.json can still differ, and its `version` field changes every release by
      # construction — so it is discounted, or this guard could never fire.
      local pkg_diff
      pkg_diff="$(git diff "${prefix}${last}" -- "$pkg_dir/package.json" \
        | grep -E '^[-+]' | grep -vE '^[-+]{3}' | grep -v '"version"' || true)"
      if [[ -z "$pkg_diff" ]]; then
        die "$pkg_dir is byte-identical to ${prefix}${last} apart from its version field — nothing to release."
      fi
    fi
    semver_gt "$version" "$last" \
      || die "$version is not greater than the last released $last (${prefix}${last})."
    say "changed since ${prefix}${last}:"
    git diff --stat "${prefix}${last}" -- "$pkg_dir" | tail -5 >&2
  else
    say "no previous ${prefix}* tag — this is the FIRST npm release of ${NPM_PACKAGE}."
  fi

  verify_generated_matches_contract ts

  # The published version comes from package.json, so it is written BEFORE packing — and committed,
  # because a published package whose version exists in no commit cannot be traced back to a tree.
  local current; current="$(node -p "require('./$pkg_dir/package.json').version")"
  if [[ "$current" != "$version" ]]; then
    # A tag-triggered CI run is a DETACHED checkout of the tag, and a commit made there belongs to no
    # branch — it would publish a version whose package.json exists in no reachable tree. So the bump
    # is a PREPARE step that only runs on a branch; from a tag, the mismatch is fatal and says how to
    # fix it. Publishing an untraceable version is worse than a failed release.
    if git symbolic-ref -q HEAD >/dev/null; then
      say "setting $pkg_dir/package.json version: $current -> $version"
      run npm --prefix "$pkg_dir" version "$version" --no-git-tag-version --allow-same-version
      run git add "$pkg_dir/package.json"
      [[ -f "$pkg_dir/package-lock.json" ]] && run git add "$pkg_dir/package-lock.json"
      run git commit -m "chore(sdk): oikumenea-client $version"
    else
      die "package.json says $current but this release is $version, and HEAD is detached (a tag build) so it cannot be committed here. Bump it on a branch first: scripts/release.sh ts-sdk $version   (then tag and push)."
    fi
  fi

  say "building the package…"
  run npm --prefix "$pkg_dir" ci
  run npm --prefix "$pkg_dir" run build

  local tag="${prefix}${version}"

  if [[ $PUSH -eq 1 ]]; then
    # npm is append-only: a version can be published exactly once, and a second attempt is a hard
    # error. Checking first turns "already released" from a failed job into a no-op, which is what
    # makes it safe to run this locally AND let the tag-triggered workflow run the same command.
    if npm view "${NPM_PACKAGE}@${version}" version >/dev/null 2>&1; then
      say "${NPM_PACKAGE}@${version} is already on npm — nothing to publish."
    else
      run npm --prefix "$pkg_dir" publish --access public
    fi
    ensure_tag "$tag" "TypeScript client SDK $version"
    if ! git ls-remote --exit-code --tags origin "$tag" >/dev/null 2>&1; then
      run git push origin "$tag"
      run git push origin HEAD
    fi
    say "released: npm i ${NPM_PACKAGE}@${version}"
  else
    run npm --prefix "$pkg_dir" pack --dry-run
    say "not published (no --push). The pack listing above is exactly what would go to npm."
  fi
}

# ── check ───────────────────────────────────────────────────────────────────────────────────────
# The read-only view: what each artifact's last release was, whether it has changed since, and
# whether its generated sources still match the contract. Publishes nothing and is safe on any tree.
cmd_check() {
  local last_img last_go last_ts
  last_img="$(last_tag_matching "v")"
  last_go="$(last_tag_matching "clients/go/v")"
  last_ts="$(last_tag_matching "ts/v")"

  local s_img s_go s_ts
  if [[ -z "$last_img" ]]; then s_img="never released"
  elif tree_changed_since "v$last_img" .; then s_img="repo changed"
  else s_img="identical"; fi

  if [[ -z "$last_go" ]]; then s_go="never released"
  elif tree_changed_since "clients/go/v$last_go" clients/go; then s_go="CHANGED — a release is warranted"
  else s_go="identical — do NOT bump"; fi

  if [[ -z "$last_ts" ]]; then s_ts="never released"
  elif tree_changed_since "ts/v$last_ts" clients/typescript/src; then s_ts="CHANGED — a release is warranted"
  else s_ts="identical — do NOT bump"; fi

  printf '\n  %-14s %-12s %s\n' "ARTIFACT" "LAST" "SINCE THEN"
  printf '  %-14s %-12s %s\n' "images" "${last_img:-—}" "$s_img"
  printf '  %-14s %-12s %s\n' "go-sdk" "${last_go:-—}" "$s_go"
  printf '  %-14s %-12s %s\n\n' "ts-sdk" "${last_ts:-—}" "$s_ts"

  say "contract freshness (a stale SDK is the one failure that ships silently):"
  if scripts/gen-action-params.sh --verify >/dev/null 2>&1; then say "  go mirrors:   current"; else say "  go mirrors:   STALE — run scripts/gen-action-params.sh"; fi
  if scripts/gen-ts-client.sh --verify >/dev/null 2>&1; then say "  ts generated: current"; else say "  ts generated: STALE — run 'make sdk'"; fi

  say "npm version in package.json: $(node -p "require('./clients/typescript/package.json').version" 2>/dev/null || echo '?')"
  if git diff --quiet && git diff --cached --quiet; then say "working tree: clean"; else say "working tree: DIRTY"; fi
}

# ── argument parsing ────────────────────────────────────────────────────────────────────────────
[[ $# -ge 1 ]] || { sed -n '23,40p' "$0" >&2; exit 2; }

SUBCOMMAND="$1"; shift
VERSION=""
for arg in "$@"; do
  case "$arg" in
    --dry-run)      DRY_RUN=1 ;;
    --push)         PUSH=1 ;;
    --allow-dirty)  ALLOW_DIRTY=1 ;;
    --platforms=*)  PLATFORMS="${arg#*=}" ;;
    --registries=*) RELEASE_REGISTRIES="${arg#*=}" ;;
    -*)             die "unknown flag: $arg" ;;
    *)              [[ -z "$VERSION" ]] || die "unexpected argument: $arg"; VERSION="${arg#v}" ;;
  esac
done

case "$SUBCOMMAND" in
  check)   cmd_check ;;
  images)  [[ -n "$VERSION" ]] || die "images needs a version: scripts/release.sh images 1.2.3"; cmd_images "$VERSION" ;;
  go-sdk)  [[ -n "$VERSION" ]] || die "go-sdk needs a version: scripts/release.sh go-sdk 1.2.3";  cmd_go_sdk "$VERSION" ;;
  ts-sdk)  [[ -n "$VERSION" ]] || die "ts-sdk needs a version: scripts/release.sh ts-sdk 1.2.3";  cmd_ts_sdk "$VERSION" ;;
  *)       die "unknown subcommand '$SUBCOMMAND' (want: check | images | go-sdk | ts-sdk)" ;;
esac
