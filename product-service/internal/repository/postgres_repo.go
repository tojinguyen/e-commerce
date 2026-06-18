package repository

import (
	"context"
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

// NewPostgresRepository opens a gorm connection, logs the result, and runs AutoMigrate.
func NewPostgresRepository(dsn string, log *slog.Logger) (*PostgresRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Error("postgres connection failed", "error", err)
		return nil, err
	}
	log.Info("connected to postgres (product_db)")

	if err := db.AutoMigrate(&model.Product{}); err != nil {
		log.Error("auto-migrate failed", "error", err)
		return nil, err
	}
	return &PostgresRepository{db: db, log: log}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, p *model.Product) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*model.Product, error) {
	var p model.Product
	if err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
