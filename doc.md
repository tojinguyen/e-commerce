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

Accessible via NGINX Ingress — no port-forward needed:

| Tool | URL | Notes |
|------|-----|-------|
| Grafana | http://localhost/grafana | Login: `admin` / `admin` |
| Jaeger | http://localhost/jaeger | |
| Temporal UI | http://temporal.localhost | See note below |

> **Temporal UI:** `temporal.localhost` resolves to `127.0.0.1` in most modern browsers automatically.
> If it doesn't load, add this line to `C:\Windows\System32\drivers\etc\hosts`:
> ```
> 127.0.0.1 temporal.localhost
> ```

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
