package repository

import (
	"context"
	"errors"
	"log/slog"

	"github.com/toainguyen/ecommerce/product-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgresRepository implements WriteRepository using gorm.
type PostgresRepository struct {
	db  *gorm.DB
	log *slog.Logger
}

// NewPostgresRepository opens a gorm connection and logs the result. Schema is
// managed by the migration package (golang-migrate), not gorm AutoMigrate.
func NewPostgresRepository(dsn string, log *slog.Logger) (*PostgresRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Error("postgres connection failed", "error", err)
		return nil, err
	}
	log.Info("connected to postgres (product_db)")

	return &PostgresRepository{db: db, log: log}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, p *model.Product) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*model.Product, error) {
	var p model.Product
	if err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Update applies a full replace of the editable columns for the row with p.ID.
// It writes a column map (not the struct) so zero values like stock=0 are
// persisted and immutable columns (id, created_at) are left untouched; gorm
// refreshes updated_at automatically. Elasticsearch is reconciled via CDC.
func (r *PostgresRepository) Update(ctx context.Context, p *model.Product) error {
	res := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"sku":         p.SKU,
			"name":        p.Name,
			"description": p.Description,
			"price_cents": p.PriceCents,
			"currency":    p.Currency,
			"stock":       p.Stock,
			"attributes":  p.Attributes,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes the row by id, returning ErrNotFound when nothing matched.
// Elasticsearch is reconciled via CDC, so no ES delete is issued here.
func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&model.Product{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
