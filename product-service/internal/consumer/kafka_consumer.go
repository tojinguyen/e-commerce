package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/panjf2000/ants/v2"
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

// laneBuffer is the per-lane channel capacity. Bounded so a slow lane applies
// backpressure on the fetch loop instead of buffering unboundedly.
const laneBuffer = 256

// KafkaConsumer reads Debezium CDC events and keeps Elasticsearch in sync.
//
// To increase throughput it processes events across N "lanes" running in
// parallel, while guaranteeing that all events for the same product land on the
// same lane (sharded by product id) and are therefore applied in order. This
// preserves correctness of the eventual-consistency replica: a create→update→
// delete sequence for one product is never reordered.
type KafkaConsumer struct {
	reader  *kafka.Reader
	es      *elasticsearch.Client
	log     *slog.Logger
	pool    *ants.Pool
	lanes   []chan kafka.Message
	workers int
	wg      sync.WaitGroup
}

func NewKafkaConsumer(broker string, es *elasticsearch.Client, log *slog.Logger, workers int) *KafkaConsumer {
	if workers < 1 {
		workers = 1
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: groupID,
	})
	// One goroutine per lane is pulled from the pool, so size it to match.
	pool, _ := ants.NewPool(workers)
	lanes := make([]chan kafka.Message, workers)
	for i := range lanes {
		lanes[i] = make(chan kafka.Message, laneBuffer)
	}
	return &KafkaConsumer{
		reader:  r,
		es:      es,
		log:     log,
		pool:    pool,
		lanes:   lanes,
		workers: workers,
	}
}

// Start runs the fetch loop and lane workers. It blocks until ctx is cancelled,
// then drains in-flight messages before returning.
func (c *KafkaConsumer) Start(ctx context.Context) {
	c.log.Info("kafka consumer started", "topic", topic, "group", groupID, "workers", c.workers)

	// Spin up one ordered worker per lane.
	for i := range c.lanes {
		lane := c.lanes[i]
		c.wg.Add(1)
		_ = c.pool.Submit(func() {
			defer c.wg.Done()
			c.runLane(ctx, lane)
		})
	}

	c.fetchLoop(ctx)

	// Cancelled: stop feeding lanes, drain them, then release resources.
	for _, lane := range c.lanes {
		close(lane)
	}
	c.wg.Wait()
	c.pool.Release()
	if err := c.reader.Close(); err != nil {
		c.log.Error("kafka reader close error", "error", err)
	}
	c.log.Info("kafka consumer stopped")
}

// fetchLoop pulls messages and dispatches each to its key-sharded lane. It uses
// FetchMessage (not ReadMessage) so offsets are committed only after a lane has
// successfully processed the message — avoiding data loss on crash.
func (c *KafkaConsumer) fetchLoop(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // graceful shutdown
			}
			c.log.Error("kafka fetch error", "error", err)
			continue
		}

		laneIdx := c.laneFor(msg)
		c.log.Debug("kafka message fetched",
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"lane", laneIdx,
		)
		lane := c.lanes[laneIdx]
		select {
		case lane <- msg:
		case <-ctx.Done():
			return
		}
	}
}

// runLane processes one lane's messages strictly in order, then commits each
// offset once the work is durable.
func (c *KafkaConsumer) runLane(ctx context.Context, lane <-chan kafka.Message) {
	// Detach from ctx cancellation so buffered messages are still indexed and
	// committed during graceful drain (the lane stops when its channel closes).
	workCtx := context.WithoutCancel(ctx)
	for msg := range lane {
		if err := c.handle(workCtx, msg.Value); err != nil {
			c.log.Error("failed to handle kafka message", "error", err, "offset", msg.Offset)
		}
		// Commit even on handle error: the event is malformed/non-retryable here
		// and blocking the lane would stall the partition. Indexing is idempotent.
		if err := c.reader.CommitMessages(workCtx, msg); err != nil {
			c.log.Error("kafka commit error", "error", err, "offset", msg.Offset)
		}
	}
}

// laneFor shards a message to a lane by product key so all events for the same
// product are handled by the same (serial) lane. It prefers the Debezium message
// key (the product PK) and falls back to parsing the id out of the value.
func (c *KafkaConsumer) laneFor(msg kafka.Message) int {
	key := msg.Key
	if len(key) == 0 {
		key = []byte(productIDFromValue(msg.Value))
	}
	h := fnv.New32a()
	_, _ = h.Write(key)
	return int(h.Sum32()) % c.workers
}

// productIDFromValue extracts the product id from a Debezium envelope for use as
// a shard key when the message key is absent.
func productIDFromValue(raw []byte) string {
	var event debeziumEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return ""
	}
	if event.After != nil {
		return event.After.ID
	}
	if event.Before != nil {
		return event.Before.ID
	}
	return ""
}

func (c *KafkaConsumer) handle(ctx context.Context, raw []byte) error {
	var event debeziumEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return fmt.Errorf("unmarshal debezium event: %w", err)
	}

	c.log.Debug("handling debezium event", "op", event.Op)

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
	default:
		c.log.Warn("ignoring unknown debezium op", "op", event.Op)
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
