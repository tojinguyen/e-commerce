package config

import "os"

// Config holds runtime configuration for the cart service.
type Config struct {
	HTTPPort string
	MongoURI string
	MongoDB  string

	ServiceName  string
	OTLPEndpoint string
	Environment  string
	LogLevel     string
}

func Load() Config {
	return Config{
		HTTPPort: getenv("CART_HTTP_PORT", "8081"),
		MongoURI: getenv("CART_MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  getenv("CART_MONGO_DB", "cart_db"),

		ServiceName:  getenv("OTEL_SERVICE_NAME", "cart-service"),
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
