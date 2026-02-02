package observability

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordPushSubscriptionMetrics(t *testing.T) {
	metricsOnce = sync.Once{}
	metricsInstance = nil

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)

	if err := initMetrics(); err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	ctx := context.Background()
	RecordPushSubscriptionCreated(ctx)
	RecordPushSubscriptionDeleted(ctx)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	if got := findInt64SumMetric(t, rm, "clubhouse.push.subscriptions.created"); got != 1 {
		t.Fatalf("expected created metric to be 1, got %d", got)
	}
	if got := findInt64SumMetric(t, rm, "clubhouse.push.subscriptions.deleted"); got != 1 {
		t.Fatalf("expected deleted metric to be 1, got %d", got)
	}
}

func findInt64SumMetric(t *testing.T, rm metricdata.ResourceMetrics, name string) int64 {
	t.Helper()

	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s is not int64 sum", name)
			}
			var total int64
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
			return total
		}
	}

	t.Fatalf("metric %s not found", name)
	return 0
}
