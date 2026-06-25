// @title           Order Service API
// @version         1.0
// @description     Order management service; order lifecycle is driven by a Temporal saga workflow.
// @host            localhost
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

	productclient "github.com/toainguyen/ecommerce/order-service/internal/client/product"
	"github.com/toainguyen/ecommerce/order-service/internal/config"
	delivery "github.com/toainguyen/ecommerce/order-service/internal/delivery/http"
	_ "github.com/toainguyen/ecommerce/order-service/docs"
	"github.com/toainguyen/ecommerce/order-service/internal/migration"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"github.com/toainguyen/ecommerce/order-service/internal/uow"
	"github.com/toainguyen/ecommerce/order-service/internal/usecase"
	"github.com/toainguyen/ecommerce/pkg/observability"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
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

	// Run schema migrations before opening the repository.
	if err := migration.Run(cfg.PostgresDSN, log); err != nil {
		log.Error("fatal: migration failed", "error", err)
		os.Exit(1)
	}

	repo, db, err := repository.NewPostgresRepository(cfg.PostgresDSN, log)
	if err != nil {
		log.Error("fatal: postgres unavailable", "error", err)
		os.Exit(1)
	}
	orderUoW := uow.New(db, log)

	// Tracing interceptor injects the current trace context when starting the
	// workflow, so the saga (run on the worker) joins the create-order trace.
	tracingInterceptor, err := temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
	if err != nil {
		log.Error("fatal: temporal tracing interceptor", "error", err)
		os.Exit(1)
	}
	tc, err := client.Dial(client.Options{
		HostPort:     cfg.TemporalHostPort,
		Namespace:    cfg.TemporalNamespace,
		Interceptors: []interceptor.ClientInterceptor{tracingInterceptor},
	})
	if err != nil {
		log.Error("fatal: temporal unavailable", "error", err)
		os.Exit(1)
	}
	defer tc.Close()
	log.Info("connected to temporal", "host", cfg.TemporalHostPort, "namespace", cfg.TemporalNamespace)

	products := productclient.New(cfg.ProductServiceBaseURL)
	uc := usecase.NewOrderUsecase(repo, orderUoW, tc, cfg.TemporalTaskQueue, products, log)
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
