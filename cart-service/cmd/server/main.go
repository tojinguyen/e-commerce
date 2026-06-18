package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/toainguyen/ecommerce/cart-service/internal/config"
	delivery "github.com/toainguyen/ecommerce/cart-service/internal/delivery/http"
	"github.com/toainguyen/ecommerce/pkg/observability"
	"github.com/toainguyen/ecommerce/cart-service/internal/repository"
	"github.com/toainguyen/ecommerce/cart-service/internal/usecase"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel)
	log.Info("starting cart-service", "port", cfg.HTTPPort)

	ctx := context.Background()
	shutdownTracing, err := observability.InitTracing(ctx, cfg.ServiceName, cfg.OTLPEndpoint, cfg.Environment)
	if err != nil {
		log.Warn("tracing init failed (continuing without traces)", "error", err)
		shutdownTracing = func(context.Context) error { return nil }
	}

	repo, err := repository.NewMongoRepository(cfg.MongoURI, cfg.MongoDB, log)
	if err != nil {
		log.Error("fatal: mongodb unavailable", "error", err)
		os.Exit(1)
	}

	uc := usecase.NewCartUsecase(repo)
	handler := delivery.NewCartHandler(uc, log)
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
	log.Info("cart-service listening", "addr", srv.Addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down cart-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = shutdownTracing(shutdownCtx)
}
