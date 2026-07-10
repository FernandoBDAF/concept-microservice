# Root orchestration for the microservices monorepo.
# `make help` lists targets. Per-service Makefiles live in each service dir.

K6_IMAGE   := grafana/k6:0.54.0
K6_RUN     := docker run --rm -i --network microservices_default \
	-e API_URL=http://api-service:8080 -e AUTH_URL=http://auth-service:3000
SIM_VUS      ?= 10
SIM_DURATION ?= 2m

.PHONY: help up infra down nuke ps logs verify verify-api verify-workers verify-auth verify-graphrag \
	monitoring queues sim-smoke sim-load sim-burst sim-poison sim-outage scale demo-document

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

# ── Monitoring & simulations (PRD v1) ────────────────────────────────────────

monitoring: ## Print monitoring UI URLs
	@echo "Grafana     http://localhost:3001   (admin/admin, dashboard: Lab Overview)"
	@echo "Prometheus  http://localhost:9090   (Status → Targets to check scrapes)"
	@echo "RabbitMQ    http://localhost:15672  (guest/guest)"

queues: ## Show RabbitMQ queue depths and consumers
	docker compose exec -T rabbitmq rabbitmqctl list_queues name messages messages_ready consumers

sim-smoke: ## k6: 1 VU / 15s sanity pass through auth+API+queues
	$(K6_RUN) -e SIM_VUS=1 -e SIM_DURATION=15s $(K6_IMAGE) run - < scripts/simulate/api-load.js

sim-load: ## k6: steady load (SIM_VUS=10 SIM_DURATION=2m overridable)
	$(K6_RUN) -e SIM_VUS=$(SIM_VUS) -e SIM_DURATION=$(SIM_DURATION) $(K6_IMAGE) run - < scripts/simulate/api-load.js

sim-burst: ## k6: short burst (50 VUs / 30s)
	$(K6_RUN) -e SIM_VUS=50 -e SIM_DURATION=30s $(K6_IMAGE) run - < scripts/simulate/api-load.js

sim-poison: ## Publish malformed messages to every exchange; watch DLQs
	python3 scripts/simulate/publish.py poison --count 3
	@sleep 3 && $(MAKE) --no-print-directory queues

sim-outage: ## Stop a worker, build backlog, restart, watch drain (WORKER=email N=100)
	bash scripts/simulate/worker-outage.sh $(or $(WORKER),email) $(or $(N),100)

scale: ## Scale a service (S=email-worker N=3) — EXPERIMENTS.md EXP-07
	docker compose up -d --scale $(S)=$(N) $(S)

demo-document: ## Document pipeline E2E: upload → MinIO → graphrag (EXP-11)
	bash scripts/simulate/document-upload.sh
