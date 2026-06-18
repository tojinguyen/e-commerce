package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/toainguyen/ecommerce/order-service/internal/config"
	delivery "github.com/toainguyen/ecommerce/order-service/internal/delivery/http"
	"github.com/toainguyen/ecommerce/pkg/observability"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"github.com/toainguyen/ecommerce/order-service/internal/usecase"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel)
	log.Info("starting order-service", "port", cfg.HTTPPort)

	ctx := context.Background()
	shutdownTracing, err := observability.InitTracing(ctx, cfg.ServiceName, cfg.OTLPEndpoint, cfg.Environment)
	if err != nil {
		log.Warn("tracing init failed (continuing without traces)", "error", err)
		shutdownTracing = func(context.Context) error { return nil }
	}

	repo, err := repository.NewPostgresRepository(cfg.PostgresDSN, log)
	if err != nil {
		log.Error("fatal: postgres unavailable", "error", err)
		os.Exit(1)
	}

	tc, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalHostPort,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		log.Error("fatal: temporal unavailable", "error", err)
		os.Exit(1)
	}
	defer tc.Close()
	log.Info("connected to temporal", "host", cfg.TemporalHostPort, "namespace", cfg.TemporalNamespace)

	uc := usecase.NewOrderUsecase(repo, tc, cfg.TemporalTaskQueue, log)
	handler := delivery.NewOrderHandler(uc, log)
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
	log.Info("order-service listening", "addr", srv.Addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down order-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = shutdownTracing(shutdownCtx)
}
