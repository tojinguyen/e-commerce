package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// NewRouter wires middleware, health/metrics endpoints, and the product routes.
func NewRouter(h *ProductHandler, log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(log))
	r.Use(allowAllCORS)

	// Operational endpoints.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ready")) })
	r.Handle("/metrics", promhttp.Handler())

	// Swagger UI.
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Product API.
	r.Route("/api/v1/products", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/search", h.Search)
		r.Get("/suggest", h.Suggest)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})

	return r
}

// requestLogger logs one structured line per HTTP request with method, path,
// status, latency and request id. Operational/probe endpoints are skipped to
// keep the logs focused on real API traffic.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	skip := map[string]bool{"/healthz": true, "/readyz": true, "/metrics": true}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"remote_ip", r.RemoteAddr,
			)
		})
	}
}

// allowAllCORS permits cross-origin requests from any origin and answers
// preflight (OPTIONS) requests directly.
func allowAllCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
