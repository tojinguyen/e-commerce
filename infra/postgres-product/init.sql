-- =============================================================================
-- Product DB bootstrap + CDC (Change Data Capture) source configuration.
--
-- The server is started with wal_level=logical (see the StatefulSet args in
-- deploy/k8s/infra/postgres-product.yaml). This script prepares the publication
-- and replica identity that Debezium needs to stream row-level changes:
--     Postgres WAL -> Debezium (Kafka Connect) -> Kafka -> Elasticsearch sink.
-- =============================================================================

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku         TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT,
    price_cents BIGINT      NOT NULL DEFAULT 0,
    currency    TEXT        NOT NULL DEFAULT 'USD',
    stock       INT         NOT NULL DEFAULT 0,
    -- Dynamic, schema-less attributes (color, size, specs, ...) stored as JSONB.
    attributes  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- REPLICA IDENTITY FULL emits complete before/after row images in the WAL so the
-- downstream Elasticsearch sink can apply deletes/updates correctly.
ALTER TABLE products REPLICA IDENTITY FULL;

-- Debezium consumes this publication (plugin: pgoutput).
DROP PUBLICATION IF EXISTS ecommerce_cdc;
CREATE PUBLICATION ecommerce_cdc FOR TABLE products;
