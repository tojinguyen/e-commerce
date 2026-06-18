package model

import "time"

// CartItem is a single line in a cart.
type CartItem struct {
	ProductID string `json:"product_id" bson:"product_id"`
	SKU       string `json:"sku" bson:"sku"`
	Quantity  int    `json:"quantity" bson:"quantity"`
	UnitCents int64  `json:"unit_cents" bson:"unit_cents"`
}

// Cart is the volatile, high-write document stored in MongoDB, keyed by user ID.
type Cart struct {
	UserID    string     `json:"user_id" bson:"_id"`
	Items     []CartItem `json:"items" bson:"items"`
	UpdatedAt time.Time  `json:"updated_at" bson:"updated_at"`
}
