# Build context is the REPO ROOT (see Makefile: docker build -f cart-service/Dockerfile .)
# so the shared ../pkg module referenced via `replace` is available.
# ---- build ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY cart-service/go.mod cart-service/go.sum* ./cart-service/
RUN cd cart-service && go mod download
COPY pkg/ ./pkg/
COPY cart-service/ ./cart-service/
RUN cd cart-service && CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/server"]
