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

.PHONY: swag
swag: ## regenerate swagger docs for every service (requires swag CLI)
	cd product-service && swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
	cd cart-service && swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
	cd order-service && swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

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
	kind create cluster --name $(KIND_CLUSTER) --config deploy/kind-config.yaml
	kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	kubectl patch deployment metrics-server -n kube-system --patch-file $(K8S_DIR)/system/metrics-server-patch.yaml

.PHONY: cluster-down
cluster-down: ## delete the kind cluster
	kind delete cluster --name $(KIND_CLUSTER)

.PHONY: cluster-pause
cluster-pause: ## pause the kind cluster (saves CPU/RAM without losing state)
	docker pause $(KIND_CLUSTER)-control-plane

.PHONY: cluster-resume
cluster-resume: ## resume a paused kind cluster
	docker unpause $(KIND_CLUSTER)-control-plane

.PHONY: cluster-recover
cluster-recover: ## recover kind cluster after Docker Desktop restart (re-attaches network then starts)
	docker network connect kind $(KIND_CLUSTER)-control-plane 2>/dev/null || true
	docker start $(KIND_CLUSTER)-control-plane

## ---- Deploy ------------------------------------------------------------

.PHONY: deploy
deploy: swag images kind-load ## regenerate swagger, build images, load into kind, then apply all manifests
	kubectl apply -f $(K8S_DIR)/00-namespace.yaml
	kubectl apply -f $(K8S_DIR)/infra/
	kubectl apply -f $(K8S_DIR)/apps/
	$(MAKE) restart
	kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=180s
	kubectl apply -f $(K8S_DIR)/ingress/
	kubectl apply -f $(K8S_DIR)/observability/
	$(MAKE) pf

.PHONY: pf
pf: ## background port-forward all DBs + search + Jaeger (OTLP+UI) + Temporal UI
	kubectl -n ecommerce wait --for=condition=ready pod -l app=postgres-product --timeout=180s
	kubectl -n ecommerce wait --for=condition=ready pod -l app=postgres-order --timeout=180s
	kubectl -n ecommerce wait --for=condition=ready pod -l app=mongodb --timeout=180s
	kubectl -n ecommerce wait --for=condition=ready pod -l app=elasticsearch --timeout=180s
	kubectl -n ecommerce wait --for=condition=ready pod -l app=kibana --timeout=180s
	kubectl -n observability wait --for=condition=ready pod -l app=jaeger --timeout=180s
	kubectl -n ecommerce wait --for=condition=ready pod -l app=temporal-ui --timeout=180s
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/postgres-product 5433:5432
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/postgres-order 5434:5432
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/mongodb 27017:27017
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/elasticsearch 9200:9200
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/kibana 5601:5601
	cmd /c start "" /min kubectl -n observability port-forward svc/jaeger 16686:16686 4318:4318
	cmd /c start "" /min kubectl -n ecommerce port-forward svc/temporal-ui 8088:8080

.PHONY: restart
restart: ## force pods to pick up freshly side-loaded :$(TAG) images (same-tag rebuilds don't change the manifest, so apply alone won't roll)
	kubectl -n ecommerce rollout restart deployment/product-service
	kubectl -n ecommerce rollout restart deployment/cart-service
	kubectl -n ecommerce rollout restart deployment/order-service

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

.PHONY: pf-ingress
pf-ingress: ## Ingress -> http://localhost:80 (workaround when cluster lacks extraPortMappings)
	kubectl -n ingress-nginx port-forward svc/ingress-nginx-controller 80:80

.PHONY: pf-product
pf-product: ## product-service swagger -> http://localhost:8080/swagger/index.html
	kubectl -n ecommerce port-forward svc/product-service 8080:8080

.PHONY: pf-es
pf-es: ## Elasticsearch REST API -> http://localhost:9200 (e.g. /products/_search?pretty)
	kubectl -n ecommerce port-forward svc/elasticsearch 9200:9200

.PHONY: pf-kibana
pf-kibana: ## Kibana UI -> http://localhost:5601 (Discover + Dev Tools)
	kubectl -n ecommerce port-forward svc/kibana 5601:5601

.PHONY: pf-grafana
pf-grafana: ## Grafana -> http://localhost:3000
	kubectl -n observability port-forward svc/grafana 3000:3000

.PHONY: pf-jaeger
pf-jaeger: ## Jaeger UI + OTLP receiver -> http://localhost:16686 (traces: localhost:4318)
	kubectl -n observability port-forward svc/jaeger 16686:16686 4318:4318

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
	@echo   cluster-pause       pause the kind cluster (saves CPU/RAM)
	@echo   cluster-resume      resume a paused kind cluster
	@echo   cluster-recover     recover cluster after Docker Desktop restart
	@echo   deploy              build images, load into kind, apply all manifests
	@echo   restart             rollout restart app deployments to pick up new :$(TAG) images
	@echo   undeploy            delete all manifests
	@echo   validate            client-side dry-run of all manifests
	@echo   swag                regenerate swagger docs for every service
	@echo   register-connector  register the Debezium Postgres CDC source
	@echo   pf                  background port-forward all DBs + ES + Kibana + Jaeger + Temporal UI
	@echo   pf-ingress          port-forward ingress  (localhost:80) — temp workaround
	@echo   pf-product          port-forward product-service swagger (localhost:8080)
	@echo   pf-es               port-forward Elasticsearch (localhost:9200)
	@echo   pf-kibana           port-forward Kibana UI (localhost:5601)
	@echo   pf-grafana          port-forward Grafana  (localhost:3000)
	@echo   pf-jaeger           port-forward Jaeger   (localhost:16686)
	@echo   pf-temporal         port-forward Temporal (localhost:8088)
