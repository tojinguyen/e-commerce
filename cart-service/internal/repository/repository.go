package repository

import (
	"context"

	"github.com/toainguyen/ecommerce/cart-service/internal/model"
)

// CartRepository is the persistence port for carts (MongoDB).
type CartRepository interface {
	Upsert(ctx context.Context, cart *model.Cart) error
	Get(ctx context.Context, userID string) (*model.Cart, error)
}
