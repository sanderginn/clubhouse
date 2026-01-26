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
	httpRequestCount          metric.Int64Counter
	httpRequestDuration       metric.Float64Histogram
	websocketConnections      metric.Int64UpDownCounter
	websocketConnectsTotal    metric.Int64Counter
	websocketDisconnectsTotal metric.Int64Counter
	websocketMessagesReceived metric.Int64Counter
	websocketMessagesSent     metric.Int64Counter
	websocketSubscriptionsAdd metric.Int64Counter
	websocketSubscriptionsRem metric.Int64Counter
	websocketErrors           metric.Int64Counter
	postsCreated              metric.Int64Counter
	commentsCreated           metric.Int64Counter
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

		// WebSocket message/subscribe/error metrics:
		// - clubhouse.websocket.messages.received (attrs: message_type)
		// - clubhouse.websocket.messages.sent (attrs: message_type)
		// - clubhouse.websocket.subscriptions.added (attrs: message_type)
		// - clubhouse.websocket.subscriptions.removed (attrs: message_type)
		// - clubhouse.websocket.errors (attrs: message_type, error_type)
		websocketMessagesReceived, err := meter.Int64Counter(
			"clubhouse.websocket.messages.received",
			metric.WithDescription("Count of websocket messages received"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketMessagesSent, err := meter.Int64Counter(
			"clubhouse.websocket.messages.sent",
			metric.WithDescription("Count of websocket messages sent"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketSubscriptionsAdd, err := meter.Int64Counter(
			"clubhouse.websocket.subscriptions.added",
			metric.WithDescription("Count of websocket subscriptions added"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketSubscriptionsRem, err := meter.Int64Counter(
			"clubhouse.websocket.subscriptions.removed",
			metric.WithDescription("Count of websocket subscriptions removed"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		websocketErrors, err := meter.Int64Counter(
			"clubhouse.websocket.errors",
			metric.WithDescription("Count of websocket message handling errors"),
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
			websocketMessagesReceived: websocketMessagesReceived,
			websocketMessagesSent:     websocketMessagesSent,
			websocketSubscriptionsAdd: websocketSubscriptionsAdd,
			websocketSubscriptionsRem: websocketSubscriptionsRem,
			websocketErrors:           websocketErrors,
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

// RecordWebsocketMessageReceived increments the received message counter.
func RecordWebsocketMessageReceived(ctx context.Context, messageType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.websocketMessagesReceived.Add(ctx, 1, metric.WithAttributes(attribute.String("message_type", messageType)))
}

// RecordWebsocketMessageSent increments the sent message counter.
func RecordWebsocketMessageSent(ctx context.Context, messageType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.websocketMessagesSent.Add(ctx, 1, metric.WithAttributes(attribute.String("message_type", messageType)))
}

// RecordWebsocketSubscriptionAdded increments the subscription added counter.
func RecordWebsocketSubscriptionAdded(ctx context.Context, messageType string, count int) {
	m := getMetrics()
	if m == nil {
		return
	}
	if count <= 0 {
		return
	}
	m.websocketSubscriptionsAdd.Add(ctx, int64(count), metric.WithAttributes(attribute.String("message_type", messageType)))
}

// RecordWebsocketSubscriptionRemoved increments the subscription removed counter.
func RecordWebsocketSubscriptionRemoved(ctx context.Context, messageType string, count int) {
	m := getMetrics()
	if m == nil {
		return
	}
	if count <= 0 {
		return
	}
	m.websocketSubscriptionsRem.Add(ctx, int64(count), metric.WithAttributes(attribute.String("message_type", messageType)))
}

// RecordWebsocketError increments the websocket error counter.
func RecordWebsocketError(ctx context.Context, errorType, messageType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("error_type", errorType),
		attribute.String("message_type", messageType),
	}
	m.websocketErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
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
