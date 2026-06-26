// Package product is an HTTP client for the product-service catalog API. It is
// used by the order usecase to verify item prices against the source of truth
// before an order is persisted, instead of trusting client-supplied prices.
package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// ErrInsufficientStock is returned by AdjustStock when the product-service
// rejects the delta because stock would go below zero.
var ErrInsufficientStock = errors.New("insufficient stock")

// ErrProductNotFound is returned by AdjustStock when the product does not exist.
var ErrProductNotFound = errors.New("product not found")

// Product is the subset of the catalog product needed for price verification.
type Product struct {
	ID         string `json:"id"`
	SKU        string `json:"sku"`
	PriceCents int64  `json:"price_cents"`
	Currency   string `json:"currency"`
	Stock      int    `json:"stock"`
}

// Client talks to the product-service over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client for the given product-service base URL (e.g. http://localhost:8080).
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		// otelhttp transport emits a client span for each request and injects the
		// trace headers, linking this call into the order-service trace.
		http: &http.Client{
			Timeout:   5 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

// GetProducts fetches the given product ids in a single batch request and returns
// them keyed by id. Ids that do not exist are simply absent from the map, so the
// caller is responsible for detecting missing products.
func (c *Client) GetProducts(ctx context.Context, ids []string) (map[string]*Product, error) {
	body, err := json.Marshal(map[string][]string{"ids": ids})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/products/batch", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product-service returned status %d", resp.StatusCode)
	}

	var out struct {
		Products []Product `json:"products"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}

	m := make(map[string]*Product, len(out.Products))
	for i := range out.Products {
		m[out.Products[i].ID] = &out.Products[i]
	}
	return m, nil
}

// AdjustStock sends a signed delta to the product-service stock endpoint.
// Use a negative delta to reserve units (inventory decrease on order) and a
// positive delta to release them (compensation on order failure).
func (c *Client) AdjustStock(ctx context.Context, productID string, delta int) error {
	body, err := json.Marshal(map[string]int{"delta": delta})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
		c.baseURL+"/api/v1/products/"+productID+"/stock",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrProductNotFound
	case http.StatusConflict:
		return ErrInsufficientStock
	default:
		return fmt.Errorf("product-service returned status %d", resp.StatusCode)
	}
}
