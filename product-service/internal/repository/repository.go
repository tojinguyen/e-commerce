package repository

import (
	"context"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
)

// WriteRepository is the persistence port for the catalog source of truth (Postgres).
type WriteRepository interface {
	Create(ctx context.Context, p *model.Product) error
	GetByID(ctx context.Context, id string) (*model.Product, error)
}

// SearchRepository is the read/search port backed by Elasticsearch. It is kept
// separate from WriteRepository because the two are populated asynchronously via CDC.
type SearchRepository interface {
	Search(ctx context.Context, query string, size int) ([]model.SearchResult, error)
	Suggest(ctx context.Context, prefix string, size int) ([]string, error)
}
