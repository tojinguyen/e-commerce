package repository

import (
	"context"
	"log/slog"

	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PostgresRepository implements OrderRepository using gorm.
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
	log.Info("connected to postgres (order_db)")

	return &PostgresRepository{db: db, log: log}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, o *model.Order) error {
	return r.db.WithContext(ctx).Create(o).Error
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*model.Order, error) {
	var o model.Order
	if err := r.db.WithContext(ctx).First(&o, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Order{}).
		Where("id = ?", id).
		Update("status", status).Error
}
