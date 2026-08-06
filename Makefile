# go-oikumenea — task runner for developers and operators.
#
# Run `make` (or `make help`) to see every target. These are a friendly front door: they delegate to
# the tools that actually own each job — the bundled gödel wrapper (./godelw) for build/test/format/
# lint, Atlas for migrations, and docker compose for the local stack — so there is no second source
# of truth to drift.

# Load .env (if present) so Atlas / tests / compose see DATABASE_URL and friends. `export` passes them
# to recipe sub-shells; a real environment variable still wins over the .env value.
-include .env
export

GODEL          ?= ./godelw
ATLAS          ?= atlas
COMPOSE        ?= docker compose
DEV_COMPOSE    ?= docker-compose.dev.yml
OIKU_IMAGE     ?= oikumenea:local
HERMENEA_IMAGE ?= hermenea:local
DEMO_DSN       ?= $(if $(DATABASE_URL),$(DATABASE_URL),postgres://postgres:dev@localhost:5432/postgres?sslmode=disable)

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------------------------------
##@ General
# ---------------------------------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

# ---------------------------------------------------------------------------------------------------
##@ Build
# ---------------------------------------------------------------------------------------------------

.PHONY: build
build: ## Build the oikumenea + hermenea binaries (-> out/build/)
	$(GODEL) build oikumenea hermenea

.PHONY: install
install: ## Install oikumenea + hermenea into $GOBIN (on your PATH)
	go install ./cmd/oikumenea ./cmd/hermenea

.PHONY: clean
clean: ## Remove build/dist outputs
	$(GODEL) clean || true
	rm -rf out

# ---------------------------------------------------------------------------------------------------
##@ Code generation (Conjure is the API source of truth — never hand-edit generated code)
# ---------------------------------------------------------------------------------------------------

.PHONY: generate
generate: ## Regenerate Go server/client code from api/*.conjure.yml
	$(GODEL) conjure

.PHONY: openapi
openapi: ## Regenerate the OpenAPI reference (docs/api/openapi/openapi.json)
	./scripts/gen-openapi.sh

.PHONY: sdk
sdk: ## Regenerate the TypeScript SDK (clients/typescript)
	./scripts/gen-ts-client.sh

# ---------------------------------------------------------------------------------------------------
##@ Quality (format, lint, test)
# ---------------------------------------------------------------------------------------------------

.PHONY: format
format: ## Format all Go files (gofmt + imports, via gödel)
	$(GODEL) format

.PHONY: lint
lint: ## Run the linters (golangci-lint, via gödel)
	$(GODEL) lint

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: test
test: ## Run unit tests
	$(GODEL) test

.PHONY: test-integration
test-integration: ## Run integration tests (needs the test DBs — run `make test-db` first)
	go test -tags=integration ./internal/...

.PHONY: verify
verify: ## Run the full check bundle (format-check + lint + test + license) — what CI runs
	$(GODEL) verify

# ---------------------------------------------------------------------------------------------------
##@ Run (from source)
# ---------------------------------------------------------------------------------------------------

.PHONY: run
run: ## Run the oikumenea server (API :8443, management :8444)
	go run ./cmd/oikumenea serve

.PHONY: run-hermenea
run-hermenea: ## Run the hermenea companion (API :9443, management :9444)
	go run ./cmd/hermenea

.PHONY: bootstrap-admin
bootstrap-admin: ## Seed / recover the first instance admin (break-glass, D-Bootstrap)
	go run ./cmd/oikumenea bootstrap-admin

# ---------------------------------------------------------------------------------------------------
##@ Database & migrations
# ---------------------------------------------------------------------------------------------------

.PHONY: dev-up
dev-up: ## Start the local dev stack (Postgres + Keycloak)
	$(COMPOSE) -f $(DEV_COMPOSE) up -d

.PHONY: dev-down
dev-down: ## Stop the local dev stack
	$(COMPOSE) -f $(DEV_COMPOSE) down

.PHONY: migrate
migrate: ## Apply oikumenea migrations to the dev DB (Atlas env local)
	$(ATLAS) migrate apply --env local

.PHONY: migrate-hermenea
migrate-hermenea: ## Apply hermenea migrations to its own DB (Atlas env hermenea)
	$(ATLAS) migrate apply --env hermenea

.PHONY: migrate-status
migrate-status: ## Show applied vs pending migrations
	$(ATLAS) migrate status --env local

.PHONY: migrate-lint
migrate-lint: ## Destructive-change gate on pending migrations (needs Docker)
	$(ATLAS) migrate lint --env local

.PHONY: db-reset
db-reset: ## Reset the dev DB to a clean, migrated state + re-seed the admin
	./scripts/reset-dev-db.sh

.PHONY: test-db
test-db: ## Create + migrate the dedicated integration-test databases (idempotent)
	./scripts/setup-test-db.sh

.PHONY: seed-demo
seed-demo: ## Seed the dev DB with realistic demo data (unit trees, ranked persons, families, all modules). Needs a migrated DB + pinax seeded (run the server once or `oikumenea seed`).
	go run ./scripts/seed-demo -dsn "$(DEMO_DSN)" -reset

# ---------------------------------------------------------------------------------------------------
##@ Docker
# ---------------------------------------------------------------------------------------------------

.PHONY: docker-build
docker-build: ## Build the oikumenea container image (oikumenea:local)
	docker build -f Dockerfile -t $(OIKU_IMAGE) .

.PHONY: docker-build-hermenea
docker-build-hermenea: ## Build the hermenea container image (hermenea:local)
	docker build -f Dockerfile.hermenea -t $(HERMENEA_IMAGE) .

.PHONY: docker-up
docker-up: ## Start the packaged stack (add `--profile ui` for the web console)
	$(COMPOSE) up -d --build

.PHONY: docker-down
docker-down: ## Stop the packaged stack
	$(COMPOSE) down

# ---------------------------------------------------------------------------------------------------
##@ Release (see docs/releasing.md — each artifact has its OWN version and its OWN tag)
# ---------------------------------------------------------------------------------------------------

.PHONY: release-check
release-check: ## What each artifact would release right now (publishes nothing)
	scripts/release.sh check

.PHONY: release-images
release-images: ## Build the three images at VERSION for both registries (PUSH=1 to publish, PLATFORMS= to narrow)
	@[ -n "$(VERSION)" ] || { echo "usage: make release-images VERSION=1.2.3 [PUSH=1] [PLATFORMS=linux/amd64]"; exit 2; }
	scripts/release.sh images $(VERSION) $(if $(PUSH),--push,) $(if $(PLATFORMS),--platforms=$(PLATFORMS),)

.PHONY: release-go-sdk
release-go-sdk: ## Verify + tag the nested Go SDK module at VERSION (add PUSH=1 to publish)
	@[ -n "$(VERSION)" ] || { echo "usage: make release-go-sdk VERSION=1.2.3 [PUSH=1]"; exit 2; }
	scripts/release.sh go-sdk $(VERSION) $(if $(PUSH),--push,)

.PHONY: release-ts-sdk
release-ts-sdk: ## Verify + publish oikumenea-client at VERSION to npm (add PUSH=1 to publish)
	@[ -n "$(VERSION)" ] || { echo "usage: make release-ts-sdk VERSION=1.2.3 [PUSH=1]"; exit 2; }
	scripts/release.sh ts-sdk $(VERSION) $(if $(PUSH),--push,)

# ---------------------------------------------------------------------------------------------------
##@ Web console (optional Next.js admin UI)
# ---------------------------------------------------------------------------------------------------

.PHONY: web-install
web-install: ## Install the web console's npm dependencies
	cd web && npm install

.PHONY: web-dev
web-dev: ## Run the web console in dev mode (http://localhost:8445)
	cd web && npm run dev

.PHONY: web-build
web-build: ## Production build of the web console
	cd web && npm run build
