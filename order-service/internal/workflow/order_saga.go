package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// OrderInput is the workflow argument.
type OrderInput struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	TotalCents int64  `json:"total_cents"`
}

// OrderResult is the workflow return value.
type OrderResult struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// OrderWorkflow orchestrates the order saga. Each step is an activity; on failure
// the already-completed steps are compensated in reverse order, then the order is
// marked FAILED. On success it is marked CONFIRMED.
func OrderWorkflow(ctx workflow.Context, in OrderInput) (OrderResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	log := workflow.GetLogger(ctx)
	var a Activities

	// Track compensations to run on failure (LIFO).
	var compensations []func()

	// 1) Reserve inventory.
	if err := workflow.ExecuteActivity(ctx, a.ReserveInventory, in).Get(ctx, nil); err != nil {
		return fail(ctx, in, "reserve inventory failed", err, compensations)
	}
	compensations = append(compensations, func() {
		_ = workflow.ExecuteActivity(ctx, a.ReleaseInventory, in).Get(ctx, nil)
	})

	// 2) Process payment.
	if err := workflow.ExecuteActivity(ctx, a.ProcessPayment, in).Get(ctx, nil); err != nil {
		return fail(ctx, in, "payment failed", err, compensations)
	}
	compensations = append(compensations, func() {
		_ = workflow.ExecuteActivity(ctx, a.RefundPayment, in).Get(ctx, nil)
	})

	// 3) Create shipment.
	if err := workflow.ExecuteActivity(ctx, a.CreateShipment, in).Get(ctx, nil); err != nil {
		return fail(ctx, in, "shipment failed", err, compensations)
	}

	// 4) Confirm.
	if err := workflow.ExecuteActivity(ctx, a.MarkConfirmed, in).Get(ctx, nil); err != nil {
		return OrderResult{OrderID: in.OrderID, Status: "UNKNOWN"}, err
	}
	log.Info("order saga completed", "order_id", in.OrderID)
	return OrderResult{OrderID: in.OrderID, Status: "CONFIRMED"}, nil
}

// fail runs compensations in reverse, marks the order FAILED, and returns the error.
func fail(ctx workflow.Context, in OrderInput, msg string, cause error, comps []func()) (OrderResult, error) {
	log := workflow.GetLogger(ctx)
	log.Error(msg, "order_id", in.OrderID, "error", cause)
	for i := len(comps) - 1; i >= 0; i-- {
		comps[i]()
	}
	var a Activities
	_ = workflow.ExecuteActivity(ctx, a.MarkFailed, in).Get(ctx, nil)
	return OrderResult{OrderID: in.OrderID, Status: "FAILED"}, cause
}
