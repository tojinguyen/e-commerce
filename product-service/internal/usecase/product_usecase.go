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

// UpdateProduct replaces the editable fields of an existing product in Postgres
// and returns the persisted record (with refreshed timestamps). As with create,
// Elasticsearch is updated asynchronously via the CDC pipeline — no dual write.
func (u *ProductUsecase) UpdateProduct(ctx context.Context, p *model.Product) (*model.Product, error) {
	if err := u.write.Update(ctx, p); err != nil {
		return nil, err
	}
	return u.write.GetByID(ctx, p.ID)
}

// DeleteProduct removes a product from the source of truth. The CDC pipeline
// propagates the deletion to Elasticsearch.
func (u *ProductUsecase) DeleteProduct(ctx context.Context, id string) error {
	return u.write.Delete(ctx, id)
}

func (u *ProductUsecase) Search(ctx context.Context, params model.SearchParams) ([]model.SearchResult, error) {
	if params.Size <= 0 {
		params.Size = 10
	}
	return u.search.Search(ctx, params)
}

func (u *ProductUsecase) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	if size <= 0 {
		size = 5
	}
	return u.search.Suggest(ctx, prefix, size)
}
