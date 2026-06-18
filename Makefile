# ============================================================================
# E-commerce Microservices — Makefile
# Build images, side-load into kind, apply manifests, and drive local dev.
# ============================================================================

SERVICES      := product-service cart-service order-service
REGISTRY      ?= ecommerce
TAG           ?= dev
KIND_CLUSTER  ?= ecommerce
K8S_DIR       := deploy/k8s

.DEFAULT_GOAL := help

## ---- Go ----------------------------------------------------------------

.PHONY: tidy
tidy: ## go mod tidy for every service
	@for s in $(SERVICES); do echo ">> tidy $$s"; (cd $$s && go mod tidy); done

.PHONY: build
build: ## go build ./... for every service
	@for s in $(SERVICES); do echo ">> build $$s"; (cd $$s && go build ./...); done

.PHONY: work
work: ## (re)generate go.work
	go work use ./product-service ./cart-service ./order-service

## ---- Docker images -----------------------------------------------------

.PHONY: images
images: ## build a docker image per service
	@for s in $(SERVICES); do \
		echo ">> docker build $$s"; \
		docker build -t $(REGISTRY)/$$s:$(TAG) ./$$s; \
	done

.PHONY: kind-load
kind-load: ## side-load images into the kind cluster (no registry needed)
	@for s in $(SERVICES); do \
		echo ">> kind load $$s"; \
		kind load docker-image $(REGISTRY)/$$s:$(TAG) --name $(KIND_CLUSTER); \
	done

## ---- Cluster lifecycle -------------------------------------------------

.PHONY: cluster-up
cluster-up: ## create a local kind cluster + ingress + metrics-server
	kind create cluster --name $(KIND_CLUSTER) || true
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	@echo "NOTE: on kind, patch metrics-server with --kubelet-insecure-tls if it CrashLoops."

.PHONY: cluster-down
cluster-down: ## delete the kind cluster
	kind delete cluster --name $(KIND_CLUSTER)

## ---- Deploy ------------------------------------------------------------

.PHONY: deploy
deploy: ## apply all manifests (namespaces -> infra -> apps -> ingress -> observability)
	kubectl apply -f $(K8S_DIR)/00-namespace.yaml
	kubectl apply -f $(K8S_DIR)/infra/
	kubectl apply -f $(K8S_DIR)/apps/
	kubectl apply -f $(K8S_DIR)/ingress/
	kubectl apply -f $(K8S_DIR)/observability/

.PHONY: undeploy
undeploy: ## delete all manifests
	-kubectl delete -f $(K8S_DIR)/observability/
	-kubectl delete -f $(K8S_DIR)/ingress/
	-kubectl delete -f $(K8S_DIR)/apps/
	-kubectl delete -f $(K8S_DIR)/infra/
	-kubectl delete -f $(K8S_DIR)/00-namespace.yaml

.PHONY: validate
validate: ## client-side dry-run of every manifest
	kubectl apply --dry-run=client -R -f $(K8S_DIR)/

## ---- CDC / Debezium ----------------------------------------------------

.PHONY: register-connector
register-connector: ## register the Debezium Postgres source connector via Connect REST API
	kubectl -n ecommerce exec deploy/kafka-connect -- \
		curl -s -X POST -H "Content-Type: application/json" \
		--data @/dev/stdin http://localhost:8083/connectors \
		< $(K8S_DIR)/infra/debezium/register-postgres-connector.json

## ---- Port-forwards (run in separate terminals) -------------------------

.PHONY: pf-grafana
pf-grafana: ## Grafana -> http://localhost:3000
	kubectl -n observability port-forward svc/grafana 3000:3000

.PHONY: pf-jaeger
pf-jaeger: ## Jaeger UI -> http://localhost:16686
	kubectl -n observability port-forward svc/jaeger 16686:16686

.PHONY: pf-temporal
pf-temporal: ## Temporal UI -> http://localhost:8088
	kubectl -n ecommerce port-forward svc/temporal-ui 8088:8080

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
