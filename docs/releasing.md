# Releasing — the three artifacts, their versions, and how to publish them

This deployment publishes **three artifacts**, and they are versioned **independently**:

| Artifact | Where it goes | Version lives in | Released by |
|---|---|---|---|
| **Container images** — `oikumenea`, `hermenea`, `oikumenea-console` | `ghcr.io/olehmushka/*` **and** `docker.io/olehmushka/*` | the tag alone | tag `v1.2.3` |
| **Go SDK** — the nested `clients/go` module | nothing is uploaded; the module proxy serves the tag | the tag alone | tag `clients/go/v1.2.3` |
| **TypeScript SDK** — `oikumenea-client` | npmjs.com (public) | `clients/typescript/package.json` | tag `ts/v1.2.3` |

**Why independent versions.** Both SDKs are *generated from the same Conjure contract as the server*
([D-ClientSDK](architecture/decisions.md)), so most releases of the server change nothing in them. A
lockstep version would publish an empty SDK on every server release — and every consumer would have
to read a diff to discover it was empty. So an SDK version moves **only when its artifact does**, and
that rule is enforced rather than remembered: `scripts/release.sh` **refuses** to publish an SDK
whose sources are byte-identical to its last release.

Everything runs through one script — [`scripts/release.sh`](../scripts/release.sh) — which
[`.github/workflows/release.yml`](../.github/workflows/release.yml) calls with the same arguments you
would type. Nothing is published by logic that exists only inside a workflow file, so a release that
misbehaves in CI can be reproduced on a laptop with `--dry-run`.

## Before anything: what would happen?

```bash
make release-check          # or: scripts/release.sh check
```

It prints each artifact's last release, whether it has changed since, and whether the generated
sources still match `api/*.conjure.yml`. It publishes nothing and is safe on any tree.

```
  ARTIFACT       LAST         SINCE THEN
  images         1.4.0        repo changed
  go-sdk         0.3.1        identical — do NOT bump
  ts-sdk         0.3.1        CHANGED — a release is warranted
```

## Releasing the images

```bash
scripts/release.sh images 1.2.3            # build all three for amd64+arm64, push nothing
git tag -a v1.2.3 -m "…" && git push origin v1.2.3   # CI builds and pushes
```

Each image gets three tags per registry — `1.2.3`, `1.2` (the patch line an operator can track) and
`latest` — plus OCI labels carrying the version, the commit and the source URL. **Pin a digest in
anything that matters**; the moving tags are a convenience, not a contract.

A local run without `--push` builds to `type=cacheonly`: a multi-platform build cannot be loaded into
the local daemon (the image store holds one platform per tag), so the dry run proves every Dockerfile
compiles for every platform and then throws the result away. That is the useful half.

**Only the console needs emulation.** The two Go images pin their build stage to `$BUILDPLATFORM` and
cross-compile to `$TARGETARCH`, which is free — CGO is off and pgx is pure Go. The console cannot: its
`npm run build` runs in a target-arch container, so the arm64 leg genuinely runs under QEMU and is the
slow part of a release.

That means a laptop without binfmt handlers registered can build every image for amd64, and the two Go
images for arm64, but not the console for arm64. The script checks up front and says so, rather than
letting it surface 200 lines into a build as `exec /bin/sh: exec format error`:

```bash
docker run --privileged --rm tonistiigi/binfmt --install all   # once per machine
scripts/release.sh images 1.2.3 --platforms=linux/amd64        # …or just build one platform
```

CI installs the handlers itself (`docker/setup-qemu-action`), so this only ever bites locally.

## Releasing the Go SDK

```bash
scripts/release.sh go-sdk 0.4.0            # guards + build/vet/test, creates the tag locally
scripts/release.sh go-sdk 0.4.0 --push     # …and pushes it
```

A Go module release **is** the tag — nothing is uploaded, and `proxy.golang.org` serves it within
minutes. The tag form `clients/go/vX.Y.Z` is fixed by the nested module's path and is not a choice.

> **BLOCKED today.** The module declares `github.com/olegamysk/go-oikumenea/clients/go`, and that path
> returns 404 — the repository lives at `github.com/olehmushka/go-oikumenea`. The proxy fetches a
> module by its **go.mod path**, not by the repo you tagged, so tagging now would publish a module
> that `go get` cannot install, and the version number would be spent. `release.sh go-sdk` refuses
> for exactly this reason. Resolve it by renaming the module path to match the repository (go.mod,
> clients/go/go.mod and every import), or by making the declared path resolve.

The command refuses to proceed when `clients/go` is byte-identical to the last release, when the
version does not move forward, when the IR-derived mirrors are stale against the contract, or when
the module does not build/vet/test clean.

## Releasing the TypeScript SDK

The npm version lives in `package.json`, so it must be **committed before the tag** — a published
version whose `package.json` exists in no reachable tree cannot be traced back to a commit:

```bash
scripts/release.sh ts-sdk 0.4.0            # bumps package.json, commits, builds, packs (no publish)
git push && git tag -a ts/v0.4.0 -m "…" && git push origin ts/v0.4.0   # CI publishes
```

Run from a **branch**, not a detached tag checkout: the bump needs a commit to land on, and the
script fails with that message rather than publishing something untraceable.

`npm publish` is append-only, so the script checks whether the version is already on the registry and
treats "already published" as a no-op — which is what makes it safe to run locally *and* let the
tag-triggered workflow run the identical command.

## The one failure that ships silently

Both SDK commands verify the committed generated sources against `api/*.conjure.yml`
(`scripts/gen-action-params.sh --verify`, `scripts/gen-ts-client.sh --verify`) before publishing.

A stale SDK is not a broken build — it compiles, type-checks and imports cleanly. It simply describes
an API the server does not serve, and the consumer finds out at runtime. That check is why the
release path runs the generators rather than trusting that someone ran `make sdk`.

## Credentials

CI needs three repository secrets; the built-in `GITHUB_TOKEN` covers GHCR.

| Secret | Used for |
|---|---|
| `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | pushing to `docker.io/olehmushka/*` |
| `NPM_TOKEN` | publishing `oikumenea-client` (an **automation** token, so 2FA does not block CI) |

Locally, `docker login ghcr.io`, `docker login docker.io` and `npm login` are enough — the script
never handles a credential itself.

## Manual dispatch

The workflow also takes a `workflow_dispatch` with an artifact picker, a version and a `dry_run`
toggle (**default on**), for when you want CI's environment without pushing a tag first.
