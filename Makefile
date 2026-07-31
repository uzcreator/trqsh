# trqsh — developer tunneling SaaS. Build specs live in plan/.
# On Windows, run these with `make` (via scoop/choco/git-bash) or execute the
# underlying commands directly. See docs/DEVELOPMENT.md.

GO     ?= go
MODULE := github.com/trqsh-uz/trqsh
COMPOSE := docker compose -f deploy/docker-compose.dev.yml

.PHONY: help proto build test lint tidy dev dev-deps dev-web dev-down observability \
        migrate images helm-lint helm-template tf-validate compose-config run-edge run-agent \
        site site-build site-plans site-openapi openapi-sync

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

proto: ## Regenerate protobuf code (needs proto/*.proto from Part 01)
	protoc --go_out=. --go_opt=module=$(MODULE) proto/*.proto

build: ## Build all Go binaries
	$(GO) build ./...

test: ## Run all Go tests with the race detector
	$(GO) test ./... -race -count=1

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## Tidy go modules
	$(GO) mod tidy

dev: ## Start the full local stack (pg, redis, migrate, api, edge, mailhog)
	$(COMPOSE) up -d --build

dev-deps: ## Start just Postgres + Redis (for `go run` workflows)
	$(COMPOSE) up -d postgres redis

dev-web: ## Full stack + the Next.js dashboard (profile: web)
	$(COMPOSE) --profile web up -d --build

observability: ## Add Prometheus + Grafana + OTel (profile: observability)
	$(COMPOSE) --profile observability up -d

dev-down: ## Stop the local stack (all profiles)
	$(COMPOSE) --profile web --profile observability down

migrate: ## Apply DB migrations against the compose Postgres
	$(COMPOSE) run --rm migrate up

images: ## Build all container images locally
	docker build -f deploy/docker/Dockerfile.edge      -t trqsh/edge .
	docker build -f deploy/docker/Dockerfile.api       -t trqsh/api .
	docker build -f deploy/docker/Dockerfile.migrate   -t trqsh/migrate .
	docker build -f deploy/docker/Dockerfile.dashboard -t trqsh/dashboard .

helm-lint: ## Lint the Helm chart
	helm lint deploy/helm/trqsh -f deploy/helm/trqsh/values.staging.yaml

helm-template: ## Render the Helm chart (must succeed)
	helm template trqsh deploy/helm/trqsh -f deploy/helm/trqsh/values.staging.yaml \
		--set secrets.existingSecret=trqsh-secrets >/dev/null && echo "helm template OK"

tf-validate: ## terraform fmt + validate (no cloud creds needed)
	terraform -chdir=deploy/terraform fmt -check -recursive
	terraform -chdir=deploy/terraform init -backend=false
	terraform -chdir=deploy/terraform validate

compose-config: ## Validate the compose file
	$(COMPOSE) config --quiet && echo "compose OK"

run-edge: ## Run the edge server with stub entitlements (Part 02)
	TRQSH_ENTITLEMENTS=stub TRQSH_BASE_DOMAIN=lvh.me $(GO) run ./cmd/trqshd

run-agent: ## Run the agent, e.g. `make run-agent ARGS="http 3000"` (Part 03)
	$(GO) run ./cmd/trqsh $(ARGS)

site-plans: ## Regenerate web/site's pricing catalog from the control API (Part 09)
	cd web/site && node scripts/genplans.mjs

site-openapi: ## Regenerate web/site's local OpenAPI copy from the control API (Part 09)
	cd web/site && node scripts/gen-openapi.mjs

# site/site-build deliberately do NOT depend on site-plans/site-openapi: both
# fetch from a live control API (TRQSH_API_URL, default api.trqsh.uz), and a
# routine dev/build run should never require that API to be reachable — only
# explicit regeneration (above) or CI's drift check does. The checked-in
# lib/catalog.generated.ts and lib/openapi.generated.yaml are what normal
# builds actually use.
site: ## Run the marketing site dev server on :3002 (Part 09)
	cd web/site && pnpm install && pnpm dev

site-build: ## Production build of the marketing site (Part 09)
	cd web/site && pnpm install --frozen-lockfile && pnpm build

openapi-sync: ## Sync the API's embedded OpenAPI copy from docs/openapi.yaml
	cp docs/openapi.yaml internal/api/openapi.yaml
