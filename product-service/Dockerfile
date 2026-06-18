# Build context is the REPO ROOT (see Makefile: docker build -f product-service/Dockerfile .)
# so the shared ../pkg module referenced via `replace` is available.
# ---- build ----
FROM golang:1.26-alpine AS build
WORKDIR /src
# Module manifests first for better layer caching.
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY product-service/go.mod product-service/go.sum* ./product-service/
RUN cd product-service && go mod download
# Sources.
COPY pkg/ ./pkg/
COPY product-service/ ./product-service/
RUN cd product-service && CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
