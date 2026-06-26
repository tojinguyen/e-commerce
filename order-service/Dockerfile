# Build context is the REPO ROOT (see Makefile: docker build -f order-service/Dockerfile .)
# so the shared ../pkg module referenced via `replace` is available.
# Builds both the API server and the Temporal worker; the k8s worker Deployment
# overrides the entrypoint to /worker (see deploy/k8s/apps/order-service.yaml).
# ---- build ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY order-service/go.mod order-service/go.sum* ./order-service/
RUN cd order-service && go mod download
COPY pkg/ ./pkg/
COPY order-service/ ./order-service/
RUN cd order-service && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server
RUN cd order-service && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/worker ./cmd/worker

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=build /out/worker /worker
EXPOSE 8082
USER nonroot:nonroot
ENTRYPOINT ["/server"]
