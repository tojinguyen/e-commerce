package usecase

import (
	"context"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
	"github.com/toainguyen/ecommerce/product-service/internal/repository"
)

// ProductUsecase holds business logic and depends only on repository ports.
type ProductUsecase struct {
	write  repository.WriteRepository
	search repository.SearchRepository
}

func NewProductUsecase(w repository.WriteRepository, s repository.SearchRepository) *ProductUsecase {
	return &ProductUsecase{write: w, search: s}
}

// CreateProduct persists a product to the source of truth (Postgres). The CDC
// pipeline asynchronously projects it into Elasticsearch — no dual write here.
func (u *ProductUsecase) CreateProduct(ctx context.Context, p *model.Product) error {
	return u.write.Create(ctx, p)
}

func (u *ProductUsecase) Search(ctx context.Context, query string, size int) ([]model.SearchResult, error) {
	if size <= 0 {
		size = 10
	}
	return u.search.Search(ctx, query, size)
}

func (u *ProductUsecase) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	if size <= 0 {
		size = 5
	}
	return u.search.Suggest(ctx, prefix, size)
}
