package config

import "os"

// Config holds runtime configuration for the order service and its Temporal worker.
type Config struct {
	HTTPPort    string
	PostgresDSN string

	TemporalHostPort  string
	TemporalNamespace string
	TemporalTaskQueue string

	ProductServiceBaseURL string

	ServiceName  string
	OTLPEndpoint string
	Environment  string
	LogLevel     string
}

func Load() Config {
	return Config{
		HTTPPort:    getenv("ORDER_HTTP_PORT", "8082"),
		PostgresDSN: getenv("ORDER_PG_DSN", "host=localhost port=5434 user=order password=order dbname=order_db sslmode=disable"),

		TemporalHostPort:  getenv("TEMPORAL_HOST_PORT", "localhost:7233"),
		TemporalNamespace: getenv("TEMPORAL_NAMESPACE", "default"),
		TemporalTaskQueue: getenv("TEMPORAL_TASK_QUEUE", "order-task-queue"),

		ProductServiceBaseURL: getenv("PRODUCT_SERVICE_URL", "http://localhost:8080"),

		ServiceName:  getenv("OTEL_SERVICE_NAME", "order-service"),
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
