package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	productclient "github.com/toainguyen/ecommerce/order-service/internal/client/product"
	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// ProductStockClient is the port Activities uses to mutate product inventory.
// The product HTTP client satisfies this interface.
type ProductStockClient interface {
	AdjustStock(ctx context.Context, productID string, delta int) error
}

// Activities holds the order saga activities. The Repo writes terminal order
// states; Products adjusts inventory in the product catalog.
type Activities struct {
	Repo     repository.OrderRepository
	Products ProductStockClient
	Log      *slog.Logger
}

// ReserveInventory decrements stock for every item in the order. If any product
// has insufficient stock the error is marked non-retriable — retrying the same
// request would never succeed without external intervention (stock replenishment).
func (a *Activities) ReserveInventory(ctx context.Context, in OrderInput) error {
	log := activity.GetLogger(ctx)
	items, err := a.orderItems(ctx, in.OrderID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := a.Products.AdjustStock(ctx, item.ProductID, -item.Quantity); err != nil {
			log.Error("reserve stock failed",
				"product_id", item.ProductID,
				"quantity", item.Quantity,
				"error", err,
			)
			if errors.Is(err, productclient.ErrInsufficientStock) {
				return temporal.NewNonRetryableApplicationError(
					"insufficient stock for product "+item.ProductID,
					"INSUFFICIENT_STOCK",
					err,
				)
			}
			return err
		}
		log.Info("stock reserved", "product_id", item.ProductID, "quantity", item.Quantity)
		// Heartbeat lets Temporal know the activity is still alive when iterating
		// over many items; also allows cancellation between iterations.
		activity.RecordHeartbeat(ctx, item.ProductID)
	}
	return nil
}

// ReleaseInventory is the compensation for ReserveInventory: it adds stock back
// for every item. Called in the LIFO compensation chain when a later step fails.
func (a *Activities) ReleaseInventory(ctx context.Context, in OrderInput) error {
	log := activity.GetLogger(ctx)
	items, err := a.orderItems(ctx, in.OrderID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := a.Products.AdjustStock(ctx, item.ProductID, item.Quantity); err != nil {
			log.Error("release stock failed",
				"product_id", item.ProductID,
				"quantity", item.Quantity,
				"error", err,
			)
			return err
		}
		log.Info("stock released", "product_id", item.ProductID, "quantity", item.Quantity)
		activity.RecordHeartbeat(ctx, item.ProductID)
	}
	return nil
}

// ProcessPayment charges the customer for the order total. In production this
// would call a payment gateway (Stripe, VNPay, MoMo...). Stubbed to succeed so
// the saga runs end-to-end without a real payment integration.
func (a *Activities) ProcessPayment(ctx context.Context, in OrderInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "charging")
	// TODO: call payment gateway — replace stub with real integration.
	log.Info("payment processed (stub)",
		"order_id", in.OrderID,
		"user_id", in.UserID,
		"total_cents", in.TotalCents,
	)
	return nil
}

// RefundPayment is the compensation for ProcessPayment: reverses the charge.
func (a *Activities) RefundPayment(ctx context.Context, in OrderInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "refunding")
	// TODO: call payment gateway refund endpoint.
	log.Info("payment refunded (stub)",
		"order_id", in.OrderID,
		"total_cents", in.TotalCents,
	)
	return nil
}

// CreateShipment notifies a logistics provider to dispatch the order. Stubbed
// to succeed; replace with a real carrier API (GHN, GHTK, Shopee Express...).
func (a *Activities) CreateShipment(ctx context.Context, in OrderInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "dispatching")
	// TODO: call shipment provider API.
	log.Info("shipment created (stub)",
		"order_id", in.OrderID,
		"user_id", in.UserID,
	)
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

// orderItems fetches the persisted order and unmarshals its items JSON array.
func (a *Activities) orderItems(ctx context.Context, orderID string) ([]model.OrderItem, error) {
	o, err := a.Repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	var items []model.OrderItem
	if err := json.Unmarshal(o.Items, &items); err != nil {
		return nil, err
	}
	return items, nil
}
