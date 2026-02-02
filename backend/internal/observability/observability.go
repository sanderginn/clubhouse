package observability

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Init initializes OpenTelemetry for traces and metrics.
func Init(ctx context.Context) (func(context.Context) error, http.Handler, error) {
	// Create resource
	serviceName := firstNonEmpty(
		os.Getenv("OTEL_SERVICE_NAME"),
		os.Getenv("SERVICE_NAME"),
		"clubhouse",
	)
	serviceVersion := firstNonEmpty(
		os.Getenv("OTEL_SERVICE_VERSION"),
		os.Getenv("SERVICE_VERSION"),
		"0.1.0",
	)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	traceExporter, err := newTraceExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Set global trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	registry := prometheus.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: "db.client.operation.duration",
			},
			sdkmetric.Stream{
				Name:        "clubhouse_db_query_duration_seconds",
				Description: "Database query duration in seconds",
				Unit:        "s",
			},
		)),
	)
	otel.SetMeterProvider(mp)

	logExporter, err := newLogExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	logglobal.SetLoggerProvider(logProvider)

	if err := initMetrics(); err != nil {
		return nil, nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	return func(ctx context.Context) error {
		if err := logProvider.Shutdown(ctx); err != nil {
			_ = mp.Shutdown(ctx)
			_ = tp.Shutdown(ctx)
			return err
		}
		if err := mp.Shutdown(ctx); err != nil {
			_ = tp.Shutdown(ctx)
			return err
		}
		return tp.Shutdown(ctx)
	}, handler, nil
}

func newTraceExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithTimeout(5 * time.Second),
	}

	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(parseOtlpEndpoint(endpoint)))
	}

	insecure := strings.EqualFold(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"), "true")
	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint == "" {
		insecure = true
	}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(ctx, client)
}

func newLogExporter(ctx context.Context) (*otlploghttp.Exporter, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"))
	if endpoint == "" {
		endpoint = "http://loki:3100/otlp/v1/logs"
	}

	opts := []otlploghttp.Option{
		otlploghttp.WithTimeout(5 * time.Second),
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		opts = append(opts, otlploghttp.WithEndpointURL(endpoint))
	} else {
		opts = append(opts, otlploghttp.WithEndpoint(endpoint))
		if strings.EqualFold(os.Getenv("OTEL_EXPORTER_OTLP_LOGS_INSECURE"), "true") || strings.HasPrefix(endpoint, "localhost") {
			opts = append(opts, otlploghttp.WithInsecure())
		}
	}

	return otlploghttp.New(ctx, opts...)
}

func parseOtlpEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	return strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
