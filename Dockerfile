# Multi-stage build for the go-oikumenea server (M11 packaging). pgx is pure Go, so the binary is
# built CGO-free and shipped on distroless/static (no libc, non-root). Migrations are NOT run by this
# image — apply them out-of-band (atlas) as the owner/superuser before starting the server, which
# connects as the non-superuser app role (D-RLSDefenseInDepth). See docker-compose.yml.

# ---- build ----
# --platform=$BUILDPLATFORM pins the build stage to the BUILDER's architecture and cross-compiles
# from there, instead of running an emulated toolchain. pgx is pure Go and CGO is already off, so a
# cross-compile is free — where emulating the whole Go toolchain under QEMU is minutes per arch, and
# needs binfmt registered on the host at all (a bare `docker buildx build --platform linux/arm64`
# fails with `exec format error` without it). TARGETARCH/TARGETOS are supplied by buildx.
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
# Module graph first for layer caching.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/oikumenea ./cmd/oikumenea

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
# Default config baked in; operators override by mounting their own var/conf (ECV-encrypted in prod).
COPY --from=build /src/var/conf /app/var/conf
COPY --from=build /out/oikumenea /app/oikumenea
# 8443 = app API, 8444 = management (health/readiness/debug).
EXPOSE 8443 8444
USER nonroot:nonroot
ENTRYPOINT ["/app/oikumenea"]
CMD ["serve"]
