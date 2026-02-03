package observability

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordPushSubscriptionMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previousProvider)
	})

	resetMetricsForTest()
	if err := initMetrics(); err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	RecordPushSubscriptionCreated(ctx)
	RecordPushSubscriptionDeleted(ctx)

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	if got := findInt64SumMetric(t, metrics, "clubhouse.push.subscriptions.created"); got != 1 {
		t.Fatalf("expected created metric to be 1, got %d", got)
	}
	if got := findInt64SumMetric(t, metrics, "clubhouse.push.subscriptions.deleted"); got != 1 {
		t.Fatalf("expected deleted metric to be 1, got %d", got)
	}
}

func TestRecordNotificationReadMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previousProvider)
	})

	resetMetricsForTest()
	if err := initMetrics(); err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	RecordNotificationRead(ctx, "single", 1)
	RecordNotificationRead(ctx, "all", 3)

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	singleValue := findCounterValue(t, metrics, "clubhouse.notifications.read", attribute.String("action", "single"))
	if singleValue != 1 {
		t.Fatalf("expected single read count 1, got %d", singleValue)
	}

	allValue := findCounterValue(t, metrics, "clubhouse.notifications.read", attribute.String("action", "all"))
	if allValue != 3 {
		t.Fatalf("expected mark all read count 3, got %d", allValue)
	}
}

func TestRecordCSRFValidationFailureMetrics(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previousProvider)
	})

	resetMetricsForTest()
	if err := initMetrics(); err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	RecordCSRFValidationFailure(ctx, "missing")
	RecordCSRFValidationFailure(ctx, "mismatch")
	RecordCSRFValidationFailure(ctx, "expired")

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	missing := findCounterValue(t, metrics, "clubhouse.csrf.validation.failures", attribute.String("reason", "missing"))
	if missing != 1 {
		t.Fatalf("expected missing reason count 1, got %d", missing)
	}

	mismatch := findCounterValue(t, metrics, "clubhouse.csrf.validation.failures", attribute.String("reason", "mismatch"))
	if mismatch != 1 {
		t.Fatalf("expected mismatch reason count 1, got %d", mismatch)
	}

	expired := findCounterValue(t, metrics, "clubhouse.csrf.validation.failures", attribute.String("reason", "expired"))
	if expired != 1 {
		t.Fatalf("expected expired reason count 1, got %d", expired)
	}
}

func findInt64SumMetric(t *testing.T, metrics metricdata.ResourceMetrics, name string) int64 {
	t.Helper()

	for _, scope := range metrics.ScopeMetrics {
		for _, metricItem := range scope.Metrics {
			if metricItem.Name != name {
				continue
			}
			sum, ok := metricItem.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s is not int64 sum", name)
			}
			var total int64
			for _, dataPoint := range sum.DataPoints {
				total += dataPoint.Value
			}
			return total
		}
	}

	t.Fatalf("metric %s not found", name)
	return 0
}

func findCounterValue(t *testing.T, metrics metricdata.ResourceMetrics, name string, attrs ...attribute.KeyValue) int64 {
	t.Helper()

	for _, scopeMetrics := range metrics.ScopeMetrics {
		for _, metricItem := range scopeMetrics.Metrics {
			if metricItem.Name != name {
				continue
			}
			sum, ok := metricItem.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s has unexpected data type %T", name, metricItem.Data)
			}
			for _, dataPoint := range sum.DataPoints {
				if attributesMatch(dataPoint.Attributes, attrs) {
					return dataPoint.Value
				}
			}
		}
	}

	t.Fatalf("metric %s with attributes %v not found", name, attrs)
	return 0
}

func attributesMatch(set attribute.Set, attrs []attribute.KeyValue) bool {
	found := map[string]string{}
	for _, kv := range set.ToSlice() {
		found[string(kv.Key)] = kv.Value.AsString()
	}
	for _, kv := range attrs {
		if found[string(kv.Key)] != kv.Value.AsString() {
			return false
		}
	}
	return true
}

func resetMetricsForTest() {
	metricsOnce = sync.Once{}
	metricsInitErr = nil
	metricsInstance = nil
}
