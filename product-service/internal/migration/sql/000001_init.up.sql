CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku         TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT,
    price_cents BIGINT      NOT NULL DEFAULT 0,
    currency    TEXT        NOT NULL DEFAULT 'USD',
    stock       INT         NOT NULL DEFAULT 0,
    attributes  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CDC prerequisites for Debezium (pgoutput). AutoMigrate cannot express these.
ALTER TABLE products REPLICA IDENTITY FULL;
DROP PUBLICATION IF EXISTS ecommerce_cdc;
CREATE PUBLICATION ecommerce_cdc FOR TABLE products;
