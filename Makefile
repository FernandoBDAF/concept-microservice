# Root orchestration for the microservices monorepo.
# `make help` lists targets. Per-service Makefiles live in each service dir.

K6_IMAGE   := grafana/k6:0.54.0
K6_RUN     := docker run --rm -i --network microservices_default \
	-e API_URL=http://api-service:8080 -e AUTH_URL=http://auth-service:3000
SIM_VUS      ?= 10
SIM_DURATION ?= 2m

REGISTRY   := localhost:5001
TAG        ?= dev
PROFILE    ?= single

.PHONY: help up infra down nuke ps logs verify verify-api verify-workers verify-auth verify-graphrag \
	verify-relay verify-controld verify-guest \
	monitoring queues sim-smoke sim-load sim-burst sim-poison sim-outage scale demo-document \
	images init-secrets cluster-up cluster-down cluster-status cluster-logs cluster-queues \
	cluster-scale cluster-sim-smoke cluster-sim-load cluster-sim-burst drift-check \
	obs-up obs-down controld status-page guest-up guest-down guest-status \
	chaos-up chaos-down routing-keys experiment experiments

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

up: ## Build and start the full stack (infra + all services)
	bash scripts/compose/gen-jwt-keys.sh
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

verify: verify-api verify-workers verify-auth verify-graphrag verify-relay verify-controld verify-guest ## Build + test every project locally
	@echo "✅ all projects verified"

verify-api: ## Build, vet, test api-service (Go)
	cd api-service && go build ./... && go vet ./... && go test ./...

verify-workers: ## Build, vet, test operational-workers (Go)
	cd graph-worker/operational-workers && go build ./... && go vet ./... && go test ./...

verify-auth: ## Typecheck, build, test auth-service (TypeScript)
	cd auth-service && npm run typecheck && npm run build && npm test

verify-graphrag: ## Compile-check graphrag-service (Python)
	python3 -m compileall -q graph-worker/graphrag-service/src graph-worker/graphrag-service/cmd

verify-relay: ## Build, vet, test the Alertmanager→ntfy relay (Go)
	cd scripts/obs/ntfy-relay && go build ./... && go vet ./... && go test ./...

verify-controld: ## Build, vet, test lab-controld (Go)
	cd mission-control/controld && go build ./... && go vet ./... && go test ./...

verify-guest: ## Build, vet, test the hello-guest fixture (Go)
	cd guests/hello-guest && go build ./... && go vet ./... && go test ./...

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

# ── Cluster lab (PRD v2, ADR-002) ─────────────────────────────────────────────

K6_CLUSTER_RUN := docker run --rm -i \
	--add-host api.lab.local:host-gateway --add-host auth.lab.local:host-gateway \
	-e API_URL=https://api.lab.local -e AUTH_URL=https://auth.lab.local \
	-e K6_INSECURE_SKIP_TLS_VERIFY=true

images: ## Build + push all service images to the local registry (TAG=dev)
	docker build -t $(REGISTRY)/api-service:$(TAG) api-service
	docker build -t $(REGISTRY)/auth-service:$(TAG) auth-service
	docker build -t $(REGISTRY)/graphrag-service:$(TAG) graph-worker/graphrag-service
	docker build -t $(REGISTRY)/email-worker:$(TAG) -f graph-worker/operational-workers/Dockerfile.email graph-worker/operational-workers
	docker build -t $(REGISTRY)/image-worker:$(TAG) -f graph-worker/operational-workers/Dockerfile.image graph-worker/operational-workers
	docker build -t $(REGISTRY)/profile-worker:$(TAG) -f graph-worker/operational-workers/Dockerfile.profile graph-worker/operational-workers
	# loadgen: the flood generator (ADR-004.4); Dockerfile.loadgen ships with the workers
	docker build -t $(REGISTRY)/loadgen:$(TAG) -f graph-worker/operational-workers/Dockerfile.loadgen graph-worker/operational-workers
	docker build -t $(REGISTRY)/ntfy-relay:$(TAG) scripts/obs/ntfy-relay
	docker build -t $(REGISTRY)/hello-guest-web:$(TAG) --build-arg CMD=web guests/hello-guest
	docker build -t $(REGISTRY)/hello-guest-worker:$(TAG) --build-arg CMD=worker guests/hello-guest
	for i in api-service auth-service graphrag-service email-worker image-worker profile-worker loadgen ntfy-relay hello-guest-web hello-guest-worker; do \
		docker push $(REGISTRY)/$$i:$(TAG) || exit 1; done

init-secrets: ## Generate lab credentials -> k8s Secrets (ADR-009.3; FORCE=1 rotates)
	bash scripts/cluster/init-secrets.sh

cluster-up: ## kind cluster + registry + full stack (PROFILE=single|multinode)
	PROFILE=$(PROFILE) bash scripts/cluster/up.sh

cluster-down: ## Delete the kind cluster (registry survives; REGISTRY=0 removes it too)
	kind delete cluster --name lab
	@if [ "$(REGISTRY)" = "0" ]; then docker rm -f kind-registry; fi

cluster-status: ## Nodes, pods, jobs, ingresses, certificates
	@kubectl get nodes -o wide 2>/dev/null | awk '{print "  "$$1"\t"$$2"\t"$$5}' || echo "  no cluster"
	@kubectl get pods -n lab-infra -o wide 2>/dev/null
	@kubectl get pods -n lab-core -o wide 2>/dev/null
	@kubectl get jobs -n lab-infra 2>/dev/null
	@kubectl get ingress,certificate -n lab-core 2>/dev/null

cluster-logs: ## Tail a service's logs (S=api-service)
	kubectl -n lab-core logs deploy/$(S) -f

cluster-queues: ## RabbitMQ queue depths in the cluster (parity with `make queues`)
	kubectl -n lab-infra exec rabbitmq-0 -- rabbitmqctl list_queues name messages messages_ready consumers

cluster-scale: ## Scale a service in the cluster (S=email-worker N=3) — EXP-07 parity
	kubectl -n lab-core scale deploy/$(S) --replicas=$(N)

cluster-sim-smoke: ## k6 smoke against the cluster ingress (EXP-20)
	$(K6_CLUSTER_RUN) -e SIM_VUS=1 -e SIM_DURATION=15s $(K6_IMAGE) run - < scripts/simulate/api-load.js

cluster-sim-load: ## k6 steady load against the cluster ingress
	$(K6_CLUSTER_RUN) -e SIM_VUS=$(SIM_VUS) -e SIM_DURATION=$(SIM_DURATION) $(K6_IMAGE) run - < scripts/simulate/api-load.js

cluster-sim-burst: ## k6 burst against the cluster ingress
	$(K6_CLUSTER_RUN) -e SIM_VUS=50 -e SIM_DURATION=30s $(K6_IMAGE) run - < scripts/simulate/api-load.js

drift-check: ## Compose ⇄ kustomize drift check (ADR-002.4)
	python3 scripts/check-kustomize-drift.py

# ── Observability stack (PRD v3, ADR-003) ─────────────────────────────────────

obs-up: ## Observability stack into lab-obs (kps+tempo+exporters+logs+ntfy)
	bash scripts/cluster/obs-up.sh

obs-down: ## Remove the observability stack (namespace and CRDs stay)
	bash scripts/cluster/obs-down.sh

# ── Chaos engineering (PRD v4, ADR-004.3) ─────────────────────────────────────

chaos-up: ## Install Chaos Mesh into the chaos-mesh namespace (pinned; ADR-004.3)
	bash scripts/cluster/chaos-up.sh

chaos-down: ## Remove Chaos Mesh (CRDs stay; CRs deleted first)
	bash scripts/cluster/chaos-down.sh

# ── Scored experiments (PRD v4, ADR-004.1/.4) ─────────────────────────────────

routing-keys: ## Regenerate RabbitMQ definitions.json + routing-key contract tables
	python3 scripts/rabbitmq/generate-definitions.py
	@printf '%s\n%s\n\n' \
	  '<!-- GENERATED — do not edit here. Edit scripts/rabbitmq/generate-definitions.py,' \
	  '     then run `make routing-keys`. Source of truth: deploy/rabbitmq/definitions.json. -->' \
	  > graph-worker/shared/contracts/ROUTING_KEYS.md
	@cat deploy/rabbitmq/ROUTING_KEYS.generated.md >> graph-worker/shared/contracts/ROUTING_KEYS.md
	@echo "routing-keys: regenerated definitions + generated md + contract mirror"

experiment: ## Run a scored experiment (E=exp-02)
	python3 scripts/experiments/run.py $(E)

experiments: ## List scored experiments
	python3 scripts/experiments/run.py --list

# ── Mission Control seed (PRD v3→v6, ADR-001.3/ADR-005) ──────────────────────

controld: ## Run the read-only lab-controld on 127.0.0.1:4900
	cd mission-control/controld && go run .

status-page: ## Run the status page on 127.0.0.1:4901
	cd mission-control/status-page && npm install && npm run dev

# ── Guests (ADR-001.4/ADR-007, documentation/HOST_CONTRACT.md) ───────────────

G ?= hello-guest

guest-up: ## Start guest G as its own compose project (G=hello-guest)
	docker compose -f guests/$(G)/docker-compose.yml up -d --build

guest-down: ## Stop guest G
	docker compose -f guests/$(G)/docker-compose.yml down

guest-status: ## Status of guest G
	docker compose -f guests/$(G)/docker-compose.yml ps
