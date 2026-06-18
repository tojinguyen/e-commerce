package model

import (
	"time"

	"gorm.io/datatypes"
)

// Product is the catalog entity. It is the source of truth in PostgreSQL; a CDC
// pipeline (Debezium) projects changes into Elasticsearch for search/suggest.
type Product struct {
	ID          string         `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	SKU         string         `json:"sku" gorm:"uniqueIndex;not null"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description"`
	PriceCents  int64          `json:"price_cents" gorm:"not null;default:0"`
	Currency    string         `json:"currency" gorm:"not null;default:USD"`
	Stock       int            `json:"stock" gorm:"not null;default:0"`
	// Attributes stores schema-less product attributes as JSONB (color, size, specs...).
	Attributes datatypes.JSON `json:"attributes" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// TableName pins the gorm table name to match infra/postgres-product/init.sql.
func (Product) TableName() string { return "products" }

// SearchResult is a lightweight projection returned by the Elasticsearch read path.
type SearchResult struct {
	ID    string  `json:"id"`
	SKU   string  `json:"sku"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}
