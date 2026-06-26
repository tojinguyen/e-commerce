// Package observability provides shared structured logging (slog) and
// OpenTelemetry tracing bootstrap used by every service. Traces are exported over
// OTLP/HTTP to the collector (Jaeger) configured in deploy/k8s/observability.
package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// traceHandler wraps a slog.Handler and injects trace_id + span_id from the
// OTel span stored in the context. Callers must use InfoContext/ErrorContext etc.
// for the fields to appear; plain Info/Error have no context and are a no-op.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

// NewLogger returns a JSON slog logger that automatically injects trace_id and
// span_id when a context carrying an active OTel span is passed to log calls.
func NewLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(traceHandler{base})
}

// InitTracing configures a global OTLP/HTTP trace exporter and returns a shutdown
// function. If the collector is unreachable the app still runs — spans are simply
// dropped — so local development without the observability stack keeps working.
func InitTracing(ctx context.Context, serviceName, endpoint, environment string) (func(context.Context) error, error) {
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		"",
		semconv.ServiceName(serviceName),
		semconv.DeploymentEnvironmentName(environment),
	))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Register the W3C propagator so trace context flows across HTTP calls (to
	// product-service) and Temporal boundaries; without it each hop would start a
	// brand-new, disconnected trace.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
