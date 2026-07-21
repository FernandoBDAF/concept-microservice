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
	chaos-up chaos-down routing-keys experiment experiments \
	aws-init aws-plan aws-up aws-deploy aws-down aws-kubeconfig aws-sim-burst \
	aws-reaper-pack aws-ntfy-pack aws-base-pack

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

controld: ## Run lab-controld (the Mission Control control plane) on 127.0.0.1:4900
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

# ── AWS track (PRD v5, ADR-006) — see deploy/aws/README.md + AWS_SESSION.md ──

TFVARS := deploy/aws/terraform.tfvars
# region/profile parsed from tfvars so make and terraform can't disagree
AWS_TFVAR = $$(sed -n 's/^$(1)[[:space:]]*=[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}.*/\1/p' $(TFVARS))

aws-init: ## One-time: init base+session backends from bootstrap outputs (step 0)
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	@set -e; \
	BUCKET=$$(cd deploy/aws/backend-bootstrap && terraform output -raw state_bucket); \
	TABLE=$$(cd deploy/aws/backend-bootstrap && terraform output -raw lock_table); \
	REGION=$(call AWS_TFVAR,aws_region); \
	for d in base session; do \
	  echo "== terraform init deploy/aws/$$d (backend: s3://$$BUCKET)"; \
	  (cd deploy/aws/$$d && terraform init -input=false -reconfigure \
	    -backend-config="bucket=$$BUCKET" \
	    -backend-config="dynamodb_table=$$TABLE" \
	    -backend-config="region=$$REGION"); \
	done

aws-plan: ## Terraform plan for the session stack (requires step-0 setup)
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	cd deploy/aws/session && terraform plan -var-file=../terraform.tfvars

aws-up: ## Stand up a session: apply session stack, deploy, obs (~20 min)
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	cd deploy/aws/session && terraform apply -var-file=../terraform.tfvars
	$(MAKE) aws-deploy

aws-kubeconfig: ## Point kubectl at the session EKS cluster
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	@aws eks update-kubeconfig \
	  --name $$(cd deploy/aws/session && terraform output -raw cluster_name) \
	  --region $(call AWS_TFVAR,aws_region) --profile $(call AWS_TFVAR,aws_profile)

# Substitution mechanism (see deploy/k8s/overlays/aws/kustomization.yaml):
# images via `kustomize edit set image` (NOTE: mutates the tracked
# kustomization.yaml — don't commit it; same mechanism as deploy-aws.yml),
# everything else via a post-build stream sed from terraform outputs.
aws-deploy: aws-kubeconfig ## Deploy the lab onto a live session cluster (no terraform)
	@git diff --quiet -- deploy/k8s/overlays/aws/kustomization.yaml || { \
	  echo "aws-deploy: deploy/k8s/overlays/aws/kustomization.yaml is already dirty."; \
	  echo "  This target mutates it in place (kustomize edit set image) and restores"; \
	  echo "  it afterward — refusing to start on a dirty overlay so your uncommitted"; \
	  echo "  edits aren't clobbered. Commit/stash them, or:"; \
	  echo "    git checkout -- deploy/k8s/overlays/aws/kustomization.yaml"; \
	  exit 1; }
	@set -e; \
	trap 'git checkout -- deploy/k8s/overlays/aws/kustomization.yaml' EXIT; \
	TF="terraform -chdir=deploy/aws/session output -raw"; \
	ECR=$$($$TF ecr_registry); \
	TAG=$${TAG:-$$(git rev-parse --short HEAD)}; \
	echo "== images $$ECR/coppice-lab/*:$$TAG (must already be in ECR — make images REGISTRY=$$ECR/coppice-lab TAG=$$TAG, or the pipeline)"; \
	(cd deploy/k8s/overlays/aws && \
	  for i in api-service auth-service graphrag-service email-worker image-worker profile-worker; do \
	    kustomize edit set image "localhost:5001/$$i=$$ECR/coppice-lab/$$i:$$TAG"; \
	  done); \
	kustomize build --load-restrictor LoadRestrictionsNone deploy/k8s/overlays/aws \
	  | sed \
	    -e "s|AWS_REGION_PLACEHOLDER|$$($$TF region)|g" \
	    -e "s|S3_BUCKET_PLACEHOLDER|$$($$TF documents_bucket)|g" \
	    -e "s|RDS_ADDRESS_PLACEHOLDER|$$($$TF rds_address)|g" \
	    -e "s|LAB_DOMAIN_PLACEHOLDER|$$($$TF lab_domain)|g" \
	    -e "s|IRSA_API_ROLE_ARN_PLACEHOLDER|$$($$TF api_service_irsa_role_arn)|g" \
	    -e "s|IRSA_GRAPHRAG_ROLE_ARN_PLACEHOLDER|$$($$TF graphrag_service_irsa_role_arn)|g" \
	  | kubectl apply -f -
	# rabbitmq/mongo/jwt stay init-secrets-seeded on AWS; postgres-credentials
	# is ExternalSecret-owned (SKIP_POSTGRES=1 keeps hands off it)
	SKIP_POSTGRES=1 bash scripts/cluster/init-secrets.sh
	kubectl -n lab-infra wait --for=condition=complete job/rds-bootstrap --timeout=180s
	# ALB replaces ingress-nginx/cert-manager on EKS; OpenSearch off by default
	# per session (HANDOFF §7) — OBS_LOGS=1 make aws-deploy opts back in
	OBS_LOGS=$${OBS_LOGS:-0} SKIP_POSTGRES=1 bash scripts/cluster/obs-up.sh
	bash scripts/aws/session-checkpoints.sh

# Same k6 harness as `make sim-burst` (scripts/simulate/api-load.js), pointed at
# the live ALB via the API_URL/AUTH_URL override the script already honors — no
# compose network, real ACM cert. EXP-04 burst/drain against AWS.
aws-sim-burst: ## k6 burst (50 VUs/30s) against the live AWS ingress — EXP-04 on AWS
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	@set -e; \
	DOMAIN=$$(terraform -chdir=deploy/aws/session output -raw lab_domain); \
	docker run --rm -i \
	  -e API_URL=https://api.$$DOMAIN -e AUTH_URL=https://auth.$$DOMAIN \
	  -e SIM_VUS=50 -e SIM_DURATION=30s \
	  $(K6_IMAGE) run - < scripts/simulate/api-load.js

aws-down: ## Purge ingress/ALB, destroy the session stack, then assert nothing tagged remains
	@test -f $(TFVARS) || { echo "missing $(TFVARS) — see documentation/deployment/AWS_SESSION.md step 0"; exit 1; }
	# Delete Ingresses + wait for the ALB controller to reap the ALB BEFORE
	# destroy — otherwise the ALB/SGs (created outside tfstate) orphan and VPC
	# deletion fails with DependencyViolation (ADR-006.6).
	./scripts/aws/purge-ingress.sh --region $(call AWS_TFVAR,aws_region) --profile $(call AWS_TFVAR,aws_profile)
	cd deploy/aws/session && terraform destroy -var-file=../terraform.tfvars
	./scripts/aws/assert-clean.sh --region $(call AWS_TFVAR,aws_region) --profile $(call AWS_TFVAR,aws_profile)

# lambda zips must exist before the BASE stack plans/applies (validate is fine
# without them — source_code_hash is fileexists-guarded)
# Deterministic zips: copy the source into a temp dir, pin its mtime to a fixed
# epoch and drop extra attrs (-X), so an unchanged .py yields byte-identical
# bytes and terraform's source_code_hash stops churning across checkouts.
aws-reaper-pack: ## Zip the TTL reaper Lambda (HANDOFF §4)
	@set -e; d=$$(mktemp -d); cp deploy/aws/base/reaper/reaper.py "$$d/reaper.py"; \
	  touch -t 200001010000 "$$d/reaper.py"; \
	  rm -f deploy/aws/base/reaper/reaper.zip; \
	  (cd "$$d" && zip -qX reaper.zip reaper.py); \
	  mv "$$d/reaper.zip" deploy/aws/base/reaper/reaper.zip; \
	  rm -rf "$$d"

aws-ntfy-pack: ## Zip the budget→ntfy notifier Lambda (HANDOFF §3)
	@set -e; d=$$(mktemp -d); cp deploy/aws/base/ntfy-notifier/notifier.py "$$d/notifier.py"; \
	  touch -t 200001010000 "$$d/notifier.py"; \
	  rm -f deploy/aws/base/ntfy-notifier/ntfy-notifier.zip; \
	  (cd "$$d" && zip -qX ntfy-notifier.zip notifier.py); \
	  mv "$$d/ntfy-notifier.zip" deploy/aws/base/ntfy-notifier/ntfy-notifier.zip; \
	  rm -rf "$$d"

aws-base-pack: aws-reaper-pack aws-ntfy-pack ## Both base-stack lambda zips
