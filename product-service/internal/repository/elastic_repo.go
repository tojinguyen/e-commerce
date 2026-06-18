package repository

import (
	"context"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/toainguyen/ecommerce/product-service/internal/model"
)

const productIndex = "products"

// ElasticRepository implements SearchRepository against Elasticsearch.
type ElasticRepository struct {
	es  *elasticsearch.Client
	log *slog.Logger
}

// NewElasticRepository builds an ES client and pings the cluster (connection stub).
func NewElasticRepository(address string, log *slog.Logger) (*ElasticRepository, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{address},
	})
	if err != nil {
		log.Error("elasticsearch client init failed", "error", err)
		return nil, err
	}
	if res, err := es.Info(); err != nil {
		// Non-fatal: ES may still be warming up. The service starts regardless.
		log.Warn("elasticsearch ping failed (will retry on use)", "error", err)
	} else {
		_ = res.Body.Close()
		log.Info("connected to elasticsearch", "address", address)
	}
	return &ElasticRepository{es: es, log: log}, nil
}

// Search runs a full-text query against the products index.
// TODO: build a real multi_match query and decode hits. Returns a stub for now.
func (r *ElasticRepository) Search(ctx context.Context, query string, size int) ([]model.SearchResult, error) {
	r.log.Info("es search (stub)", "query", query, "size", size)
	return []model.SearchResult{
		{ID: "stub-1", SKU: "SKU-001", Name: "Stub match for: " + query, Score: 1.0},
	}, nil
}

// Suggest runs a completion/prefix suggester against the products index.
// TODO: wire the ES completion suggester. Returns a stub for now.
func (r *ElasticRepository) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	r.log.Info("es suggest (stub)", "prefix", prefix, "size", size)
	return []string{prefix + " phone", prefix + " case", prefix + " charger"}, nil
}
