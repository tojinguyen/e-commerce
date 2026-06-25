package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/toainguyen/ecommerce/product-service/internal/model"
)

const esIndex = "products"

// indexMapping defines the products index with full-text + completion fields.
var indexMapping = `{
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "sku":         { "type": "keyword" },
      "name": {
        "type": "text", "analyzer": "standard",
        "fields": { "suggest": { "type": "completion" } }
      },
      "description": { "type": "text", "analyzer": "standard" },
      "price_cents": { "type": "long" },
      "currency":    { "type": "keyword" },
      "stock":       { "type": "integer" }
    }
  }
}`

// ElasticRepository implements SearchRepository against Elasticsearch.
type ElasticRepository struct {
	es  *elasticsearch.Client
	log *slog.Logger
}

// NewElasticRepository builds an ES client and pings the cluster.
func NewElasticRepository(address string, log *slog.Logger) (*ElasticRepository, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{address},
	})
	if err != nil {
		log.Error("elasticsearch client init failed", "error", err)
		return nil, err
	}
	if res, err := es.Info(); err != nil {
		log.Warn("elasticsearch ping failed (will retry on use)", "error", err)
	} else {
		_ = res.Body.Close()
		log.Info("connected to elasticsearch", "address", address)
	}
	return &ElasticRepository{es: es, log: log}, nil
}

// Client exposes the underlying ES client for use by the Kafka consumer.
func (r *ElasticRepository) Client() *elasticsearch.Client {
	return r.es
}

// EnsureIndex creates the products index with mapping if it does not exist.
func (r *ElasticRepository) EnsureIndex(ctx context.Context) error {
	res, err := r.es.Indices.Exists([]string{esIndex}, r.es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("es check index: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		r.log.Info("elasticsearch index already exists", "index", esIndex)
		return nil
	}

	createRes, err := r.es.Indices.Create(esIndex,
		r.es.Indices.Create.WithBody(strings.NewReader(indexMapping)),
		r.es.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("es create index: %w", err)
	}
	defer createRes.Body.Close()
	if createRes.IsError() {
		return fmt.Errorf("es create index error: %s", createRes.String())
	}
	r.log.Info("elasticsearch index created", "index", esIndex)
	return nil
}

// Search runs a fuzzy full-text query against name and description, optionally
// constrained by a price range. Text matching contributes to relevance scoring
// (must), while the price bounds are applied as a non-scoring filter.
func (r *ElasticRepository) Search(ctx context.Context, params model.SearchParams) ([]model.SearchResult, error) {
	// Text clause: a fuzzy multi_match when a query is given, else match_all so a
	// pure range search (no q) still returns results.
	var must map[string]any
	if strings.TrimSpace(params.Query) != "" {
		must = map[string]any{
			"multi_match": map[string]any{
				"query":     params.Query,
				"fields":    []string{"name^3", "description"},
				"fuzziness": "AUTO",
			},
		}
	} else {
		must = map[string]any{"match_all": map[string]any{}}
	}

	bool := map[string]any{"must": must}

	// Range filter on price_cents; gte/lte are added only for the bounds provided.
	if params.MinPriceCents != nil || params.MaxPriceCents != nil {
		priceRange := map[string]any{}
		if params.MinPriceCents != nil {
			priceRange["gte"] = *params.MinPriceCents
		}
		if params.MaxPriceCents != nil {
			priceRange["lte"] = *params.MaxPriceCents
		}
		bool["filter"] = map[string]any{
			"range": map[string]any{"price_cents": priceRange},
		}
	}

	body, err := json.Marshal(map[string]any{
		"size":  params.Size,
		"query": map[string]any{"bool": bool},
	})
	if err != nil {
		return nil, fmt.Errorf("encode search query: %w", err)
	}

	res, err := r.es.Search(
		r.es.Search.WithIndex(esIndex),
		r.es.Search.WithBody(bytes.NewReader(body)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	results := make([]model.SearchResult, 0, len(raw.Hits.Hits))
	for _, hit := range raw.Hits.Hits {
		var src struct {
			SKU        string `json:"sku"`
			Name       string `json:"name"`
			PriceCents int64  `json:"price_cents"`
		}
		_ = json.Unmarshal(hit.Source, &src)
		results = append(results, model.SearchResult{
			ID:         hit.ID,
			SKU:        src.SKU,
			Name:       src.Name,
			PriceCents: src.PriceCents,
			Score:      hit.Score,
		})
	}
	return results, nil
}

// Suggest runs a completion suggester for autocomplete on product names.
func (r *ElasticRepository) Suggest(ctx context.Context, prefix string, size int) ([]string, error) {
	body := fmt.Sprintf(`{
		"suggest": {
			"name-suggest": {
				"prefix": %q,
				"completion": { "field": "name.suggest", "size": %d }
			}
		}
	}`, prefix, size)

	res, err := r.es.Search(
		r.es.Search.WithIndex(esIndex),
		r.es.Search.WithBody(strings.NewReader(body)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("es suggest: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("es suggest error: %s", res.String())
	}

	var raw struct {
		Suggest map[string][]struct {
			Options []struct {
				Text string `json:"text"`
			} `json:"options"`
		} `json:"suggest"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode suggest response: %w", err)
	}

	var suggestions []string
	for _, entry := range raw.Suggest["name-suggest"] {
		for _, opt := range entry.Options {
			suggestions = append(suggestions, opt.Text)
		}
	}

	return suggestions, nil
}
