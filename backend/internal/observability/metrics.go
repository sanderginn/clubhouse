package observability

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type metrics struct {
	httpRequestCount        metric.Int64Counter
	httpRequestDuration     metric.Float64Histogram
	websocketConnections    metric.Int64UpDownCounter
	websocketConnectsTotal  metric.Int64Counter
	websocketDisconnectsTotal metric.Int64Counter
	postsCreated            metric.Int64Counter
	commentsCreated         metric.Int64Counter
}

var (
	metricsOnce     sync.Once
	metricsInitErr  error
	metricsInstance *metrics
)

func initMetrics() error {
	metricsOnce.Do(func() {
		meter := otel.Meter("clubhouse")
		var err error

		httpRequestCount, err := meter.Int64Counter(
			"clubhouse.http.server.request.count",
			metric.WithDescription("Count of HTTP requests received"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		httpRequestDuration, err := meter.Float64Histogram(
			"clubhouse.http.server.request.duration_ms",
			metric.WithDescription("Duration of HTTP requests in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketConnections, err := meter.Int64UpDownCounter(
			"clubhouse.websocket.connections",
			metric.WithDescription("Active websocket connections"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketConnectsTotal, err := meter.Int64Counter(
			"clubhouse.websocket.connects",
			metric.WithDescription("Total websocket connection events"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketDisconnectsTotal, err := meter.Int64Counter(
			"clubhouse.websocket.disconnects",
			metric.WithDescription("Total websocket disconnection events"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		postsCreated, err := meter.Int64Counter(
			"clubhouse.posts.created",
			metric.WithDescription("Number of posts created"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		commentsCreated, err := meter.Int64Counter(
			"clubhouse.comments.created",
			metric.WithDescription("Number of comments created"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		metricsInstance = &metrics{
			httpRequestCount:          httpRequestCount,
			httpRequestDuration:       httpRequestDuration,
			websocketConnections:      websocketConnections,
			websocketConnectsTotal:    websocketConnectsTotal,
			websocketDisconnectsTotal: websocketDisconnectsTotal,
			postsCreated:              postsCreated,
			commentsCreated:           commentsCreated,
		}
	})

	return metricsInitErr
}

func getMetrics() *metrics {
	return metricsInstance
}

// RecordHTTPRequest records request count and duration.
func RecordHTTPRequest(ctx context.Context, method, route string, statusCode int, duration time.Duration) {
	m := getMetrics()
	if m == nil {
		return
	}

	attrs := []attribute.KeyValue{
		semconv.HTTPMethodKey.String(method),
		semconv.HTTPRouteKey.String(route),
		semconv.HTTPResponseStatusCodeKey.Int(statusCode),
	}

	m.httpRequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.httpRequestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// RecordWebsocketConnect increments the active connection gauge and connect counter.
func RecordWebsocketConnect(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.websocketConnections.Add(ctx, 1)
	m.websocketConnectsTotal.Add(ctx, 1)
}

// RecordWebsocketDisconnect decrements the active connection gauge and increments disconnect counter.
func RecordWebsocketDisconnect(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.websocketConnections.Add(ctx, -1)
	m.websocketDisconnectsTotal.Add(ctx, 1)
}

// RecordPostCreated increments the post created counter.
func RecordPostCreated(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.postsCreated.Add(ctx, 1)
}

// RecordCommentCreated increments the comment created counter.
func RecordCommentCreated(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.commentsCreated.Add(ctx, 1)
}
