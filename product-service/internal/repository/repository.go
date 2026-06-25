package repository

import (
	"context"
	"errors"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
)

// ErrNotFound is returned by WriteRepository when no row matches the given id.
// It lets upper layers map a missing record to HTTP 404 without importing gorm.
var ErrNotFound = errors.New("product not found")

// WriteRepository is the persistence port for the catalog source of truth (Postgres).
type WriteRepository interface {
	Create(ctx context.Context, p *model.Product) error
	GetByID(ctx context.Context, id string) (*model.Product, error)
	// GetByIDs returns the products matching the given ids. Missing ids are simply
	// omitted from the result (no error), so callers must detect absence themselves.
	GetByIDs(ctx context.Context, ids []string) ([]model.Product, error)
	Update(ctx context.Context, p *model.Product) error
	Delete(ctx context.Context, id string) error
}

// SearchRepository is the read/search port backed by Elasticsearch. It is kept
// separate from WriteRepository because the two are populated asynchronously via CDC.
type SearchRepository interface {
	Search(ctx context.Context, params model.SearchParams) ([]model.SearchResult, error)
	Suggest(ctx context.Context, prefix string, size int) ([]string, error)
}
