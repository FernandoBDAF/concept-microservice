# Root orchestration for the microservices monorepo.
# `make help` lists targets. Per-service Makefiles live in each service dir.

.PHONY: help up infra down nuke ps logs verify verify-api verify-workers verify-auth verify-graphrag

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

up: ## Build and start the full stack (infra + all services)
	docker compose up -d --build

infra: ## Start infrastructure only (postgres, redis, rabbitmq, mongodb, minio + init jobs)
	docker compose up -d postgres redis rabbitmq mongodb minio minio-init api-migrate auth-migrate

down: ## Stop the stack (keeps data volumes)
	docker compose down

nuke: ## Stop the stack and delete data volumes
	docker compose down -v

ps: ## Show stack status
	docker compose ps

logs: ## Tail logs (all, or S=<service>)
	docker compose logs -f $(S)

verify: verify-api verify-workers verify-auth verify-graphrag ## Build + test every project locally
	@echo "✅ all projects verified"

verify-api: ## Build, vet, test api-service (Go)
	cd api-service && go build ./... && go vet ./... && go test ./...

verify-workers: ## Build, vet, test operational-workers (Go)
	cd graph-worker/operational-workers && go build ./... && go vet ./... && go test ./...

verify-auth: ## Typecheck, build, test auth-service (TypeScript)
	cd auth-service && npm run typecheck && npm run build && npm test

verify-graphrag: ## Compile-check graphrag-service (Python)
	python3 -m compileall -q graph-worker/graphrag-service/src graph-worker/graphrag-service/cmd
