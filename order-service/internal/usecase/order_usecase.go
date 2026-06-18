package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"github.com/toainguyen/ecommerce/order-service/internal/workflow"
	"go.temporal.io/sdk/client"
)

// OrderUsecase persists orders and kicks off the Temporal saga.
type OrderUsecase struct {
	repo      repository.OrderRepository
	temporal  client.Client
	taskQueue string
	log       *slog.Logger
}

func NewOrderUsecase(repo repository.OrderRepository, tc client.Client, taskQueue string, log *slog.Logger) *OrderUsecase {
	return &OrderUsecase{repo: repo, temporal: tc, taskQueue: taskQueue, log: log}
}

// CreateOrder persists a PENDING order, then starts the OrderWorkflow which drives
// it to CONFIRMED/FAILED. Persistence is the source of truth; Temporal is the orchestrator.
func (u *OrderUsecase) CreateOrder(ctx context.Context, o *model.Order) (*model.Order, error) {
	o.ID = uuid.NewString()
	o.Status = model.StatusPending
	o.WorkflowID = "order-" + o.ID

	if err := u.repo.Create(ctx, o); err != nil {
		return nil, err
	}

	opts := client.StartWorkflowOptions{
		ID:        o.WorkflowID,
		TaskQueue: u.taskQueue,
	}
	in := workflow.OrderInput{OrderID: o.ID, UserID: o.UserID, TotalCents: o.TotalCents}
	we, err := u.temporal.ExecuteWorkflow(ctx, opts, workflow.OrderWorkflow, in)
	if err != nil {
		u.log.Error("failed to start order workflow", "order_id", o.ID, "error", err)
		return nil, err
	}
	u.log.Info("started order workflow", "order_id", o.ID, "workflow_id", we.GetID(), "run_id", we.GetRunID())
	return o, nil
}

func (u *OrderUsecase) GetOrder(ctx context.Context, id string) (*model.Order, error) {
	return u.repo.GetByID(ctx, id)
}
