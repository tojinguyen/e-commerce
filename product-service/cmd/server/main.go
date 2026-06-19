// @title           Product Service API
// @version         1.0
// @description     Product catalog service with Elasticsearch-backed search and autocomplete.
// @host            localhost:8080
// @BasePath        /
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/toainguyen/ecommerce/product-service/internal/config"
	delivery "github.com/toainguyen/ecommerce/product-service/internal/delivery/http"
	_ "github.com/toainguyen/ecommerce/product-service/docs"
	"github.com/toainguyen/ecommerce/product-service/internal/migration"
	"github.com/toainguyen/ecommerce/product-service/internal/repository"
	"github.com/toainguyen/ecommerce/product-service/internal/usecase"
	"github.com/toainguyen/ecommerce/pkg/observability"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel)
	log.Info("starting product-service", "port", cfg.HTTPPort)

	ctx := context.Background()
	shutdownTracing, err := observability.InitTracing(ctx, cfg.ServiceName, cfg.OTLPEndpoint, cfg.Environment)
	if err != nil {
		log.Warn("tracing init failed (continuing without traces)", "error", err)
		shutdownTracing = func(context.Context) error { return nil }
	}

	// Run schema migrations before opening repositories.
	if err := migration.Run(cfg.PostgresDSN, log); err != nil {
		log.Error("fatal: migration failed", "error", err)
		os.Exit(1)
	}

	// Repositories (adapters).
	pgRepo, err := repository.NewPostgresRepository(cfg.PostgresDSN, log)
	if err != nil {
		log.Error("fatal: postgres unavailable", "error", err)
		os.Exit(1)
	}
	esRepo, err := repository.NewElasticRepository(cfg.ESAddress, log)
	if err != nil {
		log.Error("fatal: elasticsearch client init failed", "error", err)
		os.Exit(1)
	}

	// Usecase + delivery.
	uc := usecase.NewProductUsecase(pgRepo, esRepo)
	handler := delivery.NewProductHandler(uc, log)
	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           delivery.NewRouter(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()
	log.Info("product-service listening", "addr", srv.Addr)

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down product-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = shutdownTracing(shutdownCtx)
}
