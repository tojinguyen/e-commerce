package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// NewRouter wires middleware, health/metrics endpoints, and the order routes.
func NewRouter(h *OrderHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ready")) })
	r.Handle("/metrics", promhttp.Handler())

	// Swagger UI.
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
	})

	return r
}
