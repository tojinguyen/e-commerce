package workflow

import (
	"context"

	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
)

// Activities holds the order saga activities. The repo lets terminal-state
// activities persist the order status. Inventory/payment/shipment are stubs that
// would call the respective services (Product Service, a payment gateway, etc.).
type Activities struct {
	Repo repository.OrderRepository
}

// ReserveInventory would call the Product Service to decrement stock. Stub: succeeds.
func (a *Activities) ReserveInventory(ctx context.Context, in OrderInput) error {
	return nil
}

// ReleaseInventory is the compensation for ReserveInventory. Stub: succeeds.
func (a *Activities) ReleaseInventory(ctx context.Context, in OrderInput) error {
	return nil
}

// ProcessPayment would call a payment gateway. Stub: succeeds.
func (a *Activities) ProcessPayment(ctx context.Context, in OrderInput) error {
	return nil
}

// RefundPayment is the compensation for ProcessPayment. Stub: succeeds.
func (a *Activities) RefundPayment(ctx context.Context, in OrderInput) error {
	return nil
}

// CreateShipment would call a shipping service. Stub: succeeds.
func (a *Activities) CreateShipment(ctx context.Context, in OrderInput) error {
	return nil
}

// MarkConfirmed persists the terminal CONFIRMED state.
func (a *Activities) MarkConfirmed(ctx context.Context, in OrderInput) error {
	if a.Repo == nil {
		return nil
	}
	return a.Repo.UpdateStatus(ctx, in.OrderID, model.StatusConfirmed)
}

// MarkFailed persists the terminal FAILED state.
func (a *Activities) MarkFailed(ctx context.Context, in OrderInput) error {
	if a.Repo == nil {
		return nil
	}
	return a.Repo.UpdateStatus(ctx, in.OrderID, model.StatusFailed)
}
