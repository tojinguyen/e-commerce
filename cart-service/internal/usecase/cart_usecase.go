package usecase

import (
	"context"

	"github.com/toainguyen/ecommerce/cart-service/internal/model"
	"github.com/toainguyen/ecommerce/cart-service/internal/repository"
)

type CartUsecase struct {
	repo repository.CartRepository
}

func NewCartUsecase(repo repository.CartRepository) *CartUsecase {
	return &CartUsecase{repo: repo}
}

// SaveCart upserts the cart for the given user.
func (u *CartUsecase) SaveCart(ctx context.Context, cart *model.Cart) error {
	return u.repo.Upsert(ctx, cart)
}

// GetCart fetches the cart for the given user (empty cart if none).
func (u *CartUsecase) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	return u.repo.Get(ctx, userID)
}
