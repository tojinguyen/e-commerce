package config

import (
	"os"
)

// Config holds runtime configuration sourced from the environment. In Kubernetes
// these come from the service ConfigMap/Secret (see deploy/k8s/apps/product-service.yaml).
type Config struct {
	HTTPPort    string
	PostgresDSN string
	ESAddress   string
	KafkaBroker string

	ServiceName  string
	OTLPEndpoint string
	Environment  string
	LogLevel     string
}

// Load reads configuration from environment variables, applying sane defaults
// for local development.
func Load() Config {
	return Config{
		HTTPPort:    getenv("PRODUCT_HTTP_PORT", "8080"),
		PostgresDSN: getenv("PRODUCT_PG_DSN", "host=localhost port=5433 user=product password=product dbname=product_db sslmode=disable"),
		ESAddress:   getenv("PRODUCT_ES_ADDRESS", "http://localhost:9200"),
		KafkaBroker: getenv("KAFKA_BROKER", "localhost:9092"),

		ServiceName:  getenv("OTEL_SERVICE_NAME", "product-service"),
		OTLPEndpoint: getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		Environment:  getenv("OTEL_SERVICE_ENVIRONMENT", "local"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
