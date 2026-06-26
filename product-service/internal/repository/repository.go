package repository

import (
	"context"
	"errors"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
)

// ErrNotFound is returned by WriteRepository when no row matches the given id.
// It lets upper layers map a missing record to HTTP 404 without importing gorm.
var ErrNotFound = errors.New("product not found")

// ErrInsufficientStock is returned by AdjustStock when a negative delta would
// drive stock below zero. The caller should treat this as a non-retriable error.
var ErrInsufficientStock = errors.New("insufficient stock")

// WriteRepository is the persistence port for the catalog source of truth (Postgres).
type WriteRepository interface {
	Create(ctx context.Context, p *model.Product) error
	GetByID(ctx context.Context, id string) (*model.Product, error)
	// GetByIDs returns the products matching the given ids. Missing ids are simply
	// omitted from the result (no error), so callers must detect absence themselves.
	GetByIDs(ctx context.Context, ids []string) ([]model.Product, error)
	Update(ctx context.Context, p *model.Product) error
	Delete(ctx context.Context, id string) error
	// AdjustStock atomically adds delta to the product's stock. Use a negative
	// delta to reserve units and a positive delta to release them. Returns
	// ErrInsufficientStock when stock + delta would go below zero.
	AdjustStock(ctx context.Context, id string, delta int) error
}

// SearchRepository is the read/search port backed by Elasticsearch. It is kept
// separate from WriteRepository because the two are populated asynchronously via CDC.
type SearchRepository interface {
	Search(ctx context.Context, params model.SearchParams) ([]model.SearchResult, error)
	Suggest(ctx context.Context, prefix string, size int) ([]string, error)
}
