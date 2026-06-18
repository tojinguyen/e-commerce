package model

import (
	"time"

	"gorm.io/datatypes"
)

// Order status values mirror the Temporal saga outcome.
const (
	StatusPending   = "PENDING"
	StatusConfirmed = "CONFIRMED"
	StatusFailed    = "FAILED"
)

// OrderItem is a single line in an order.
type OrderItem struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	UnitCents int64  `json:"unit_cents"`
}

// Order is persisted in PostgreSQL; its lifecycle is driven by the OrderWorkflow.
type Order struct {
	ID         string         `json:"id" gorm:"type:uuid;primaryKey"`
	UserID     string         `json:"user_id" gorm:"index;not null"`
	Status     string         `json:"status" gorm:"index;not null;default:PENDING"`
	TotalCents int64          `json:"total_cents" gorm:"not null;default:0"`
	Currency   string         `json:"currency" gorm:"not null;default:USD"`
	Items      datatypes.JSON `json:"items" gorm:"type:jsonb;not null;default:'[]'"`
	WorkflowID string         `json:"workflow_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }
