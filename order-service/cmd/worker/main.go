package main

import (
	"context"

	"github.com/toainguyen/ecommerce/order-service/internal/config"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	wf "github.com/toainguyen/ecommerce/order-service/internal/workflow"
	"github.com/toainguyen/ecommerce/pkg/observability"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

// The worker is deployed separately from the API server (its own Deployment) so it
// can scale independently. It polls the task queue and executes the OrderWorkflow
// and its activities.
func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel)
	log.Info("starting order worker", "task_queue", cfg.TemporalTaskQueue)

	// Export the worker's workflow/activity spans to Jaeger; without this the saga
	// half of the create-order trace would be dropped.
	ctx := context.Background()
	shutdownTracing, err := observability.InitTracing(ctx, cfg.ServiceName, cfg.OTLPEndpoint, cfg.Environment)
	if err != nil {
		log.Warn("tracing init failed (continuing without traces)", "error", err)
		shutdownTracing = func(context.Context) error { return nil }
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	repo, _, err := repository.NewPostgresRepository(cfg.PostgresDSN, log)
	if err != nil {
		log.Error("fatal: postgres unavailable", "error", err)
		return
	}

	// Same tracing interceptor as the API client: the worker rebuilds the trace
	// from the propagated context so its spans nest under the create-order trace.
	tracingInterceptor, err := temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
	if err != nil {
		log.Error("fatal: temporal tracing interceptor", "error", err)
		return
	}
	tc, err := client.Dial(client.Options{
		HostPort:     cfg.TemporalHostPort,
		Namespace:    cfg.TemporalNamespace,
		Interceptors: []interceptor.ClientInterceptor{tracingInterceptor},
	})
	if err != nil {
		log.Error("fatal: temporal unavailable", "error", err)
		return
	}
	defer tc.Close()
	log.Info("connected to temporal", "host", cfg.TemporalHostPort)

	w := worker.New(tc, cfg.TemporalTaskQueue, worker.Options{})
	w.RegisterWorkflow(wf.OrderWorkflow)
	w.RegisterActivity(&wf.Activities{Repo: repo})

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Error("worker stopped", "error", err)
	}
}
