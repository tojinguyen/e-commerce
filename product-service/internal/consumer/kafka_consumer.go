package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/segmentio/kafka-go"
)

const (
	topic     = "ecommerce.public.products"
	groupID   = "product-service-es-indexer"
	esIndex   = "products"
)

// debeziumEvent mirrors the Debezium JSON envelope for the products table.
type debeziumEvent struct {
	Before *productPayload `json:"before"`
	After  *productPayload `json:"after"`
	Op     string          `json:"op"` // c=create, u=update, d=delete, r=snapshot
}

type productPayload struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	Stock       int    `json:"stock"`
}

// KafkaConsumer reads Debezium CDC events and keeps Elasticsearch in sync.
type KafkaConsumer struct {
	reader *kafka.Reader
	es     *elasticsearch.Client
	log    *slog.Logger
}

func NewKafkaConsumer(broker string, es *elasticsearch.Client, log *slog.Logger) *KafkaConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: groupID,
	})
	return &KafkaConsumer{reader: r, es: es, log: log}
}

// Start runs the consume loop. It blocks until ctx is cancelled.
func (c *KafkaConsumer) Start(ctx context.Context) {
	c.log.Info("kafka consumer started", "topic", topic, "group", groupID)
	defer c.reader.Close()

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // graceful shutdown
			}
			c.log.Error("kafka read error", "error", err)
			continue
		}

		if err := c.handle(ctx, msg.Value); err != nil {
			c.log.Error("failed to handle kafka message", "error", err, "offset", msg.Offset)
		}
	}
}

func (c *KafkaConsumer) handle(ctx context.Context, raw []byte) error {
	var event debeziumEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return fmt.Errorf("unmarshal debezium event: %w", err)
	}

	switch event.Op {
	case "c", "u", "r": // create, update, snapshot read
		if event.After == nil {
			return nil
		}
		return c.index(ctx, event.After)
	case "d": // delete
		if event.Before == nil {
			return nil
		}
		return c.delete(ctx, event.Before.ID)
	}
	return nil
}

func (c *KafkaConsumer) index(ctx context.Context, p *productPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	res, err := c.es.Index(esIndex, bytes.NewReader(body),
		c.es.Index.WithDocumentID(p.ID),
		c.es.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("es index: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("es index error: %s", res.String())
	}
	c.log.Info("indexed product", "id", p.ID, "sku", p.SKU)
	return nil
}

func (c *KafkaConsumer) delete(ctx context.Context, id string) error {
	res, err := c.es.Delete(esIndex, id,
		c.es.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("es delete: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("es delete error: %s", res.String())
	}
	c.log.Info("deleted product from index", "id", id)
	return nil
}
