# Optional admin console (D-WebUI). Build context is the REPO ROOT (not ./web) because the typed
# SDK it consumes — oikumenea-client — is a local file: dependency at ../clients/typescript, outside
# the web/ dir. The SDK's per-service clients are generated from the Conjure contract (D-ClientSDK);
# here we just compile the committed sources and let Next bundle them. Produces a lean standalone
# Next.js server.
#
#   docker compose --profile ui up --build      # brings this up on :8445

# --- deps ---------------------------------------------------------------------------------
FROM node:22-alpine AS deps
WORKDIR /app
# The SDK is a file: dep of web; its package.json must be present so `npm install` can link it.
COPY clients/typescript/package.json clients/typescript/package.json
COPY web/package.json web/package.json
RUN npm install --prefix web

# --- build --------------------------------------------------------------------------------
FROM node:22-alpine AS build
WORKDIR /app
RUN apk add --no-cache bash
COPY --from=deps /app/web/node_modules web/node_modules
COPY clients/typescript clients/typescript
COPY web web
ENV NEXT_TELEMETRY_DISABLED=1
# A BUILD-TIME placeholder, and only in this stage. `next build` collects page data for every route,
# which imports lib/api/server.ts, which requires API_BASE_URL at module scope — so the build needs
# *a* value even though the setting is a RUNTIME one. It is deliberately not carried into the runner
# stage: the operator must supply the real address (`docker run -e API_BASE_URL=…`), and without it
# the container still fails fast on boot, which is the behaviour that check exists for.
#
# Until M59 this was supplied by accident — web/.env.local rode into the build context and next build
# read it — so the image depended on an untracked file that CI did not have.
ENV API_BASE_URL=https://api.invalid
# Build the SDK (its own deps + tsc) so the file: link resolves to compiled dist, then build web
# (its prebuild rebuilds the SDK too — idempotent). next build emits the standalone server, rooted
# at the repo (outputFileTracingRoot) so the SDK is bundled in.
RUN npm install --prefix clients/typescript && npm run build --prefix clients/typescript
RUN npm run build --prefix web

# --- runner -------------------------------------------------------------------------------
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# The standalone bundle is rooted at the repo, so server.js lives under web/.
COPY --from=build /app/web/public web/public
COPY --from=build /app/web/.next/standalone ./
COPY --from=build /app/web/.next/static web/.next/static
EXPOSE 3000
CMD ["node", "web/server.js"]
