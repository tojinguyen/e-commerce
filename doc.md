# E-Commerce Microservices — Local Dev Guide

## Prerequisites

- Docker Desktop
- kind
- kubectl
- make

---

## Cluster Setup

```sh
make cluster-up   # create kind cluster + ingress-nginx + metrics-server
make deploy       # build images, load into kind, apply all manifests
```

> **Note:** `make deploy` may fail on the ingress step if ingress-nginx is still starting.
> If that happens, wait ~30s then run:
> ```sh
> kubectl apply -f deploy/k8s/ingress/
> ```

---

## Swagger UI

Accessible via the ingress at `http://localhost` — no port-forward needed.

| Service | URL |
|---------|-----|
| Product Service | http://localhost/swagger/product/index.html |
| Cart Service    | http://localhost/swagger/cart/index.html    |
| Order Service   | http://localhost/swagger/order/index.html   |

---

## API Endpoints

All routed through the ingress at `http://localhost`.

| Service | Base Path |
|---------|-----------|
| Product Service | http://localhost/api/v1/products |
| Cart Service    | http://localhost/api/v1/carts    |
| Order Service   | http://localhost/api/v1/orders   |

---

## Observability

Run each in a separate terminal (port-forward required):

| Tool | Command | URL |
|------|---------|-----|
| Grafana | `make pf-grafana` | http://localhost:3000 |
| Jaeger | `make pf-jaeger` | http://localhost:16686 |
| Temporal UI | `make pf-temporal` | http://localhost:8088 |

---

## Useful Commands

```sh
make images       # rebuild all Docker images
make kind-load    # reload images into kind cluster after rebuild
make undeploy     # delete all manifests
make cluster-down # destroy the kind cluster
make validate     # dry-run all manifests
make swag         # regenerate Swagger docs (requires swag CLI)
```

### Rollout a single service after code change

```sh
docker build -t ecommerce/product-service:dev -f product-service/Dockerfile .
kind load docker-image ecommerce/product-service:dev --name ecommerce
kubectl rollout restart deployment/product-service -n ecommerce
```
