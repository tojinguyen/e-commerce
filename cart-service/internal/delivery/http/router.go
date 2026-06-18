package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter wires middleware, health/metrics endpoints, and the cart routes.
func NewRouter(h *CartHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ready")) })
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1/carts", func(r chi.Router) {
		r.Put("/", h.Upsert)
		r.Get("/", h.Get)
	})

	return r
}
