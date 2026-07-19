# Rift — developer tunneling SaaS. Build specs live in plan/.
# On Windows, run these with `make` (via scoop/choco/git-bash) or execute the
# underlying commands directly. See docs/DEVELOPMENT.md.

GO     ?= go
MODULE := github.com/rift/rift
COMPOSE := docker compose -f deploy/docker-compose.dev.yml

.PHONY: help proto build test lint tidy dev dev-deps dev-web dev-down observability \
        migrate images helm-lint helm-template tf-validate compose-config run-edge run-agent \
        site site-build site-plans openapi-sync

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
	docker build -f deploy/docker/Dockerfile.edge      -t rift/edge .
	docker build -f deploy/docker/Dockerfile.api       -t rift/api .
	docker build -f deploy/docker/Dockerfile.migrate   -t rift/migrate .
	docker build -f deploy/docker/Dockerfile.dashboard -t rift/dashboard .

helm-lint: ## Lint the Helm chart
	helm lint deploy/helm/rift -f deploy/helm/rift/values.staging.yaml

helm-template: ## Render the Helm chart (must succeed)
	helm template rift deploy/helm/rift -f deploy/helm/rift/values.staging.yaml \
		--set secrets.existingSecret=rift-secrets >/dev/null && echo "helm template OK"

tf-validate: ## terraform fmt + validate (no cloud creds needed)
	terraform -chdir=deploy/terraform fmt -check -recursive
	terraform -chdir=deploy/terraform init -backend=false
	terraform -chdir=deploy/terraform validate

compose-config: ## Validate the compose file
	$(COMPOSE) config --quiet && echo "compose OK"

run-edge: ## Run the edge server with stub entitlements (Part 02)
	RIFT_ENTITLEMENTS=stub RIFT_BASE_DOMAIN=lvh.me $(GO) run ./cmd/riftd

run-agent: ## Run the agent, e.g. `make run-agent ARGS="http 3000"` (Part 03)
	$(GO) run ./cmd/rift $(ARGS)

site-plans: ## Regenerate web/site pricing catalog from internal/billing (Part 09)
	$(GO) run ./web/site/scripts/genplans

site: site-plans ## Run the marketing site dev server on :3002 (Part 09)
	cd web/site && pnpm install && pnpm dev

site-build: site-plans ## Production build of the marketing site (Part 09)
	cd web/site && pnpm install --frozen-lockfile && pnpm build

openapi-sync: ## Sync the API's embedded OpenAPI copy from docs/openapi.yaml
	cp docs/openapi.yaml internal/api/openapi.yaml
