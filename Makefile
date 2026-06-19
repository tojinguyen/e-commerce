# ============================================================================
# E-commerce Microservices — Makefile
#
# Recipes are written to run under BOTH Windows cmd.exe (GNU Make's default
# shell when invoked from PowerShell/cmd) and POSIX sh. That means: no bash
# `for` loops, no grep/awk — just plain `cd <dir> && <cmd>` lines, which both
# shells understand.
# ============================================================================

REGISTRY      ?= ecommerce
TAG           ?= dev
KIND_CLUSTER  ?= ecommerce
K8S_DIR       := deploy/k8s

.DEFAULT_GOAL := help

## ---- Go ----------------------------------------------------------------

.PHONY: tidy
tidy: ## go mod tidy for every module
	cd pkg && go mod tidy
	cd product-service && go mod tidy
	cd cart-service && go mod tidy
	cd order-service && go mod tidy

.PHONY: build
build: ## go build ./... for every service
	cd product-service && go build ./...
	cd cart-service && go build ./...
	cd order-service && go build ./...

.PHONY: work
work: ## (re)generate go.work
	go work use ./pkg ./product-service ./cart-service ./order-service

## ---- Docker images -----------------------------------------------------

.PHONY: images
images: ## build a docker image per service (context = repo root, for shared ./pkg)
	docker build -t $(REGISTRY)/product-service:$(TAG) -f product-service/Dockerfile .
	docker build -t $(REGISTRY)/cart-service:$(TAG) -f cart-service/Dockerfile .
	docker build -t $(REGISTRY)/order-service:$(TAG) -f order-service/Dockerfile .

.PHONY: kind-load
kind-load: ## side-load images into the kind cluster (no registry needed)
	kind load docker-image $(REGISTRY)/product-service:$(TAG) --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/cart-service:$(TAG) --name $(KIND_CLUSTER)
	kind load docker-image $(REGISTRY)/order-service:$(TAG) --name $(KIND_CLUSTER)

## ---- Cluster lifecycle -------------------------------------------------

.PHONY: cluster-up
cluster-up: ## create a local kind cluster + ingress + metrics-server
	kind create cluster --name $(KIND_CLUSTER)
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	kubectl patch deployment metrics-server -n kube-system --patch-file $(K8S_DIR)/system/metrics-server-patch.yaml

.PHONY: cluster-down
cluster-down: ## delete the kind cluster
	kind delete cluster --name $(KIND_CLUSTER)

## ---- Deploy ------------------------------------------------------------

.PHONY: deploy
deploy: images kind-load ## build images, load into kind, then apply all manifests
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
	kubectl -n ecommerce exec -i deploy/kafka-connect -- curl -s -X POST -H "Content-Type: application/json" http://localhost:8083/connectors -d @- < $(K8S_DIR)/infra/debezium/register-postgres-connector.json

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
help: ## show available targets
	@echo Targets:
	@echo   tidy                go mod tidy for every module
	@echo   build               go build ./... for every service
	@echo   work                regenerate go.work
	@echo   images              build a docker image per service
	@echo   kind-load           side-load images into the kind cluster
	@echo   cluster-up          create kind cluster + ingress + metrics-server
	@echo   cluster-down        delete the kind cluster
	@echo   deploy              build images, load into kind, apply all manifests
	@echo   undeploy            delete all manifests
	@echo   validate            client-side dry-run of all manifests
	@echo   register-connector  register the Debezium Postgres CDC source
	@echo   pf-grafana          port-forward Grafana  (localhost:3000)
	@echo   pf-jaeger           port-forward Jaeger   (localhost:16686)
	@echo   pf-temporal         port-forward Temporal (localhost:8088)
