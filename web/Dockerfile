# Optional admin console (D-WebUI). Build context is ./web (the repo-root .dockerignore
# excludes /docs/, so the spec is not available here — the committed src/lib/api/schema.d.ts
# is used instead; see scripts/gen-api-types.sh). Produces a lean standalone Next.js server.
#
#   docker compose --profile ui up --build      # brings this up on :8445

# --- deps ---------------------------------------------------------------------------------
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json ./
RUN npm install

# --- build --------------------------------------------------------------------------------
FROM node:22-alpine AS build
WORKDIR /app
RUN apk add --no-cache bash
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
# prebuild runs gen-api-types.sh; with no spec in context it falls back to the committed
# schema.d.ts (copied above). Then next build emits the standalone server.
RUN npm run build

# --- runner -------------------------------------------------------------------------------
FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
EXPOSE 3000
CMD ["node", "server.js"]
