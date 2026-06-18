package main

import (
	"github.com/toainguyen/ecommerce/order-service/internal/config"
	"github.com/toainguyen/ecommerce/pkg/observability"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	wf "github.com/toainguyen/ecommerce/order-service/internal/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// The worker is deployed separately from the API server (its own Deployment) so it
// can scale independently. It polls the task queue and executes the OrderWorkflow
// and its activities.
func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel)
	log.Info("starting order worker", "task_queue", cfg.TemporalTaskQueue)

	repo, err := repository.NewPostgresRepository(cfg.PostgresDSN, log)
	if err != nil {
		log.Error("fatal: postgres unavailable", "error", err)
		return
	}

	tc, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalHostPort,
		Namespace: cfg.TemporalNamespace,
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
