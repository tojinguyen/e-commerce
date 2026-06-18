# ---- build ----
# Builds both the API server and the Temporal worker. The k8s worker Deployment
# overrides the entrypoint to /worker (see deploy/k8s/apps/order-service.yaml).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=build /out/worker /worker
EXPOSE 8082
USER nonroot:nonroot
ENTRYPOINT ["/server"]
