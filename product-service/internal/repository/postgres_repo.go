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

// GetByIDs fetches all products whose id is in ids with a single IN query.
// Missing ids are silently absent from the result; an empty input returns an
// empty slice without hitting the database.
func (r *PostgresRepository) GetByIDs(ctx context.Context, ids []string) ([]model.Product, error) {
	if len(ids) == 0 {
		return []model.Product{}, nil
	}
	var ps []model.Product
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
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

// AdjustStock atomically applies delta to the product's stock using a single
// UPDATE with a guard clause (stock + delta >= 0). If RowsAffected == 0 the
// row either does not exist or has insufficient stock — a second COUNT
// distinguishes the two cases so the caller gets the right sentinel error.
func (r *PostgresRepository) AdjustStock(ctx context.Context, id string, delta int) error {
	res := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ? AND stock + ? >= 0", id, delta).
		Update("stock", gorm.Expr("stock + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var count int64
		r.db.WithContext(ctx).Model(&model.Product{}).Where("id = ?", id).Count(&count)
		if count == 0 {
			return ErrNotFound
		}
		return ErrInsufficientStock
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
