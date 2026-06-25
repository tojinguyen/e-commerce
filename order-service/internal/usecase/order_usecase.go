package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	productclient "github.com/toainguyen/ecommerce/order-service/internal/client/product"
	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"github.com/toainguyen/ecommerce/order-service/internal/uow"
	"github.com/toainguyen/ecommerce/order-service/internal/workflow"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.temporal.io/sdk/client"
	"gorm.io/datatypes"
)

// tracer for manual business-layer spans so the trace shows the create-order
// steps (verify pricing, persist, start workflow) between the HTTP and DB spans.
var tracer = otel.Tracer("order-usecase")

// Verification errors map to HTTP 400 in the delivery layer: they signal a bad
// request (stale/forged price, unknown product, currency mismatch) rather than
// a server fault.
var (
	ErrProductNotFound  = errors.New("product not found")
	ErrCurrencyMismatch = errors.New("item currency does not match order currency")
)

// ProductPricer is the port the usecase uses to fetch authoritative product
// prices from the catalog (product-service). Kept as an interface so the usecase
// is testable with a fake. Products are fetched in a single batch keyed by id;
// ids absent from the map do not exist.
type ProductPricer interface {
	GetProducts(ctx context.Context, ids []string) (map[string]*productclient.Product, error)
}

// OrderUsecase persists orders and kicks off the Temporal saga.
type OrderUsecase struct {
	repo      repository.OrderRepository // read-only ops (GetOrder)
	uow       *uow.OrderUoW             // transactional write (CreateOrder)
	temporal  client.Client
	taskQueue string
	products  ProductPricer
	log       *slog.Logger
}

func NewOrderUsecase(repo repository.OrderRepository, uow *uow.OrderUoW, tc client.Client, taskQueue string, products ProductPricer, log *slog.Logger) *OrderUsecase {
	return &OrderUsecase{repo: repo, uow: uow, temporal: tc, taskQueue: taskQueue, products: products, log: log}
}

// CreateOrder persists a PENDING order, then starts the OrderWorkflow which drives
// it to CONFIRMED/FAILED. Persistence is the source of truth; Temporal is the orchestrator.
func (u *OrderUsecase) CreateOrder(ctx context.Context, o *model.Order) (*model.Order, error) {
	ctx, span := tracer.Start(ctx, "OrderUsecase.CreateOrder")
	defer span.End()
	span.SetAttributes(
		attribute.String("order.user_id", o.UserID),
		attribute.String("order.currency", o.Currency),
	)

	// Verify prices against the catalog (source of truth) and derive the total
	// server-side before persisting. Client-supplied unit prices are never trusted.
	if err := u.verifyPricing(ctx, o); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "verify pricing failed")
		return nil, err
	}

	o.ID = uuid.NewString()
	o.Status = model.StatusPending
	o.WorkflowID = "order-" + o.ID
	span.SetAttributes(
		attribute.String("order.id", o.ID),
		attribute.Int64("order.total_cents", o.TotalCents),
	)

	opts := client.StartWorkflowOptions{
		ID:        o.WorkflowID,
		TaskQueue: u.taskQueue,
	}
	in := workflow.OrderInput{OrderID: o.ID, UserID: o.UserID, TotalCents: o.TotalCents}

	// Wrap DB insert + workflow start in one transaction: if ExecuteWorkflow
	// returns an error the INSERT is rolled back — no orphaned PENDING orders.
	var we client.WorkflowRun
	if err := u.uow.Run(ctx, func(txRepo repository.OrderRepository) error {
		if err := txRepo.Create(ctx, o); err != nil {
			return err
		}
		var txErr error
		we, txErr = u.temporal.ExecuteWorkflow(ctx, opts, workflow.OrderWorkflow, in)
		return txErr
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "create order transaction failed")
		u.log.Error("create order failed", "order_id", o.ID, "error", err)
		return nil, err
	}
	u.log.Info("started order workflow", "order_id", o.ID, "workflow_id", we.GetID(), "run_id", we.GetRunID())
	return o, nil
}

// verifyPricing replaces each item's unit price with the authoritative catalog
// price, rejects unknown products and currency mismatches, and recomputes the
// order total. On success o.Items (with corrected prices) and o.TotalCents are
// overwritten so the persisted order is never based on client-supplied prices.
func (u *OrderUsecase) verifyPricing(ctx context.Context, o *model.Order) error {
	ctx, span := tracer.Start(ctx, "verifyPricing")
	defer span.End()

	var items []model.OrderItem
	if err := json.Unmarshal(o.Items, &items); err != nil {
		return err
	}
	span.SetAttributes(attribute.Int("order.item_count", len(items)))

	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ProductID
	}
	products, err := u.products.GetProducts(ctx, ids)
	if err != nil {
		return err
	}

	var totalCents int64
	for i := range items {
		p, ok := products[items[i].ProductID]
		if !ok {
			return ErrProductNotFound
		}
		if p.Currency != o.Currency {
			return ErrCurrencyMismatch
		}
		items[i].UnitCents = p.PriceCents
		totalCents += p.PriceCents * int64(items[i].Quantity)
	}

	corrected, err := json.Marshal(items)
	if err != nil {
		return err
	}
	o.Items = datatypes.JSON(corrected)
	o.TotalCents = totalCents
	return nil
}

func (u *OrderUsecase) GetOrder(ctx context.Context, id string) (*model.Order, error) {
	return u.repo.GetByID(ctx, id)
}
