package repository

import (
	"context"
	"log/slog"

	"github.com/toainguyen/ecommerce/order-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// PostgresRepository implements OrderRepository using gorm.
type PostgresRepository struct {
	db  *gorm.DB
	log *slog.Logger
}

// NewPostgresRepository opens a gorm connection and logs the result. Schema is
// managed by the migration package (golang-migrate), not gorm AutoMigrate.
// It also returns the raw *gorm.DB so the caller can construct an OrderUoW.
func NewPostgresRepository(dsn string, log *slog.Logger) (*PostgresRepository, *gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Error("postgres connection failed", "error", err)
		return nil, nil, err
	}
	log.Info("connected to postgres (order_db)")

	// Emit a span per query (Create/GetByID/UpdateStatus) carrying the SQL; the
	// repo methods already pass ctx via WithContext so spans attach to the caller.
	if err := db.Use(tracing.NewPlugin(tracing.WithoutMetrics())); err != nil {
		log.Error("gorm tracing plugin failed", "error", err)
		return nil, nil, err
	}

	return &PostgresRepository{db: db, log: log}, db, nil
}

// NewPostgresRepositoryTx creates a repository backed by an already-open GORM
// transaction. Only used by OrderUoW.Run — do not call directly.
func NewPostgresRepositoryTx(tx *gorm.DB, log *slog.Logger) *PostgresRepository {
	return &PostgresRepository{db: tx, log: log}
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
