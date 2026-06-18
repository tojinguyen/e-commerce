package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter wires middleware, health/metrics endpoints, and the product routes.
func NewRouter(h *ProductHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Operational endpoints.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ready")) })
	r.Handle("/metrics", promhttp.Handler())

	// Product API.
	r.Route("/api/v1/products", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/search", h.Search)
		r.Get("/suggest", h.Suggest)
	})

	return r
}
