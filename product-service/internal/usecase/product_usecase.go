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

// GetProduct returns a single product by id from the source of truth (Postgres).
// It backs service-to-service price/currency verification (e.g. order-service).
func (u *ProductUsecase) GetProduct(ctx context.Context, id string) (*model.Product, error) {
	return u.write.GetByID(ctx, id)
}

// GetProducts returns the products matching ids in a single batch. It backs
// order-service price verification for multi-item orders, avoiding N round trips.
func (u *ProductUsecase) GetProducts(ctx context.Context, ids []string) ([]model.Product, error) {
	return u.write.GetByIDs(ctx, ids)
}

// DeleteProduct removes a product from the source of truth. The CDC pipeline
// propagates the deletion to Elasticsearch.
func (u *ProductUsecase) DeleteProduct(ctx context.Context, id string) error {
	return u.write.Delete(ctx, id)
}

// AdjustStock delegates atomic stock mutation to the repository.
// A negative delta reserves units; a positive delta releases them.
func (u *ProductUsecase) AdjustStock(ctx context.Context, id string, delta int) error {
	return u.write.AdjustStock(ctx, id, delta)
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
