# E-commerce Microservices (Go)

A simplified microservices playground for learning **horizontal scaling, SQL/NoSQL,
Temporal workflows, Elasticsearch, and CDC**. Each service uses Clean Architecture
(`delivery → usecase → repository → model`) so infrastructure stays swappable behind ports.

## Services

| Service           | Storage              | Responsibility                              | Port |
|-------------------|----------------------|---------------------------------------------|------|
| `product-service` | PostgreSQL + ES      | Catalog + inventory; search/suggest         | 8080 |
| `cart-service`    | MongoDB              | Volatile, high-write carts                  | 8081 |
| `order-service`   | PostgreSQL + Temporal| Order flow as a Saga (workflow + worker)    | 8082 |

### Endpoints (via NGINX Ingress)
- `POST /api/v1/products`, `GET /api/v1/products/search`, `GET /api/v1/products/suggest`
- `PUT /api/v1/carts`, `GET /api/v1/carts`
- `POST /api/v1/orders`, `GET /api/v1/orders/{id}`

Each service also exposes `/healthz`, `/readyz`, and `/metrics`.

## Architecture

```
            ┌──────────────┐
  client ─► │ NGINX Ingress│ ─► product / cart / order services (HPA: 2–5 replicas)
            └──────────────┘
product-service ─► PostgreSQL ─(WAL)─► Debezium ─► Kafka ─► Elasticsearch   (CDC)
cart-service    ─► MongoDB
order-service   ─► PostgreSQL  +  Temporal Cluster (OrderWorkflow saga + worker)

Observability: app /metrics + kube-state-metrics + node-exporter ─► Prometheus ─► Grafana
               OpenTelemetry traces ─► Jaeger ;  pod logs ─► Promtail ─► Loki ─► Grafana
```

## Layout

```
product-service/ cart-service/ order-service/   # Go apps (per-service go.mod + go.work)
  cmd/server   cmd/worker (order only)
  internal/{config,delivery/http,usecase,repository,model,observability,workflow}
deploy/k8s/{infra,apps,ingress,observability}    # raw Kubernetes manifests (+ Debezium connector)
api-gateway/nginx.conf.template                   # reference (Ingress is primary)
```

## Quick start (kind)

```bash
make cluster-up          # kind cluster + NGINX ingress + metrics-server
make images kind-load    # build service images, side-load into kind
make deploy              # apply namespaces -> infra -> apps -> ingress -> observability
kubectl get pods -A      # wait for Ready
make register-connector  # register the Debezium Postgres CDC source

# smoke test
curl localhost/api/v1/products/search?q=phone
curl -X PUT  localhost/api/v1/carts  -H 'X-User-ID: u1' -d '{"items":[{"sku":"SKU-1","quantity":2}]}'
curl -X POST localhost/api/v1/orders -d '{"user_id":"u1","total_cents":1999}'
```

### Observability / scaling

```bash
make pf-grafana    # http://localhost:3000  (admin/admin) — Scaling Overview dashboard
make pf-jaeger     # http://localhost:16686 — order saga traces
make pf-temporal   # http://localhost:8088  — workflow executions
kubectl get hpa -n ecommerce -w   # watch replicas scale under load
```

## Local development

```bash
make tidy && make build   # go mod tidy + go build ./... per service
make validate             # kubectl client-side dry-run of all manifests
```

> Stubs: handlers return mock/minimal data; saga activities are no-ops that drive the
> state machine. No auth. Schema is managed by golang-migrate: SQL migrations are
> embedded in each service binary (`internal/migration/sql`) and applied at startup
> from `main.go`. The product migration also sets up the Debezium CDC publication.
