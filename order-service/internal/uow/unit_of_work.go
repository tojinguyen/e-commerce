package uow

import (
	"context"
	"log/slog"

	"github.com/toainguyen/ecommerce/order-service/internal/repository"
	"gorm.io/gorm"
)

// OrderUoW wraps a *gorm.DB so that a create-order operation (DB insert +
// Temporal workflow start) can execute atomically: if either step fails the
// DB transaction is rolled back, preventing orphaned PENDING orders.
type OrderUoW struct {
	db  *gorm.DB
	log *slog.Logger
}

func New(db *gorm.DB, log *slog.Logger) *OrderUoW {
	return &OrderUoW{db: db, log: log}
}

// Run executes fn inside a single DB transaction. GORM commits on nil return
// and rolls back on any error, including errors from non-DB operations
// (e.g. Temporal ExecuteWorkflow) that are passed back through fn.
func (u *OrderUoW) Run(ctx context.Context, fn func(repo repository.OrderRepository) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(repository.NewPostgresRepositoryTx(tx, u.log))
	})
}
