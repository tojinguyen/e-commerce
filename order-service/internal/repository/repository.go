package repository

import (
	"context"

	"github.com/toainguyen/ecommerce/order-service/internal/model"
)

// OrderRepository is the persistence port for orders (Postgres).
type OrderRepository interface {
	Create(ctx context.Context, o *model.Order) error
	GetByID(ctx context.Context, id string) (*model.Order, error)
	UpdateStatus(ctx context.Context, id, status string) error
}
