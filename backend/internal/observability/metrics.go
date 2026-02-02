package observability

import (
	"context"
	"database/sql"
	"strings"
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
	authAttempts              metric.Int64Counter
	authFailures              metric.Int64Counter
	authSessionsCreated       metric.Int64Counter
	authSessionsExpired       metric.Int64Counter
	authTotpVerifications     metric.Int64Counter
	authPasswordResets        metric.Int64Counter
	ratelimitViolations       metric.Int64Counter
	ratelimitLockouts         metric.Int64Counter
	ratelimitCacheKeys        metric.Int64Counter
	postsCreated              metric.Int64Counter
	commentsCreated           metric.Int64Counter
	reactionsAdded            metric.Int64Counter
	reactionsRemoved          metric.Int64Counter
	postsDeleted              metric.Int64Counter
	postsRestored             metric.Int64Counter
	commentsDeleted           metric.Int64Counter
	commentsRestored          metric.Int64Counter
	notificationsCreated      metric.Int64Counter
	notificationsDelivered    metric.Int64Counter
	notificationsFailed       metric.Int64Counter
	linkMetadataFetchAttempts metric.Int64Counter
	linkMetadataFetchSuccess  metric.Int64Counter
	linkMetadataFetchFailures metric.Int64Counter
	linkMetadataFetchDuration metric.Float64Histogram
	searchQueries             metric.Int64Counter
	searchResults             metric.Int64Histogram
	searchDuration            metric.Float64Histogram
	cacheHits                 metric.Int64Counter
	cacheMisses               metric.Int64Counter
	uploadAttempts            metric.Int64Counter
	uploadSize                metric.Float64Histogram
	adminActions              metric.Int64Counter
	adminAuditLogViews        metric.Int64Counter
	sectionsViews             metric.Int64Counter
	postsUpdated              metric.Int64Counter
	commentsUpdated           metric.Int64Counter
	frontendWebVitals         metric.Float64Histogram
	frontendApiDuration       metric.Float64Histogram
	frontendWebsocketDuration metric.Float64Histogram
	frontendAssetDuration     metric.Float64Histogram
	frontendComponentDuration metric.Float64Histogram
	dbConnectionsOpen         metric.Int64UpDownCounter
	dbConnectionsInUse        metric.Int64UpDownCounter
	dbConnectionsIdle         metric.Int64UpDownCounter
	dbConnectionWaitCount     metric.Int64Counter
	dbConnectionWaitDuration  metric.Float64Counter
	dbQueryErrors             metric.Int64Counter
	dbTransactions            metric.Int64Counter
}

var (
	metricsOnce     sync.Once
	metricsInitErr  error
	metricsInstance *metrics
	dbStatsMu       sync.Mutex
	dbStatsSnapshot dbStatsState
)

type dbStatsState struct {
	initialized bool
	open        int64
	inUse       int64
	idle        int64
	waitCount   int64
	waitSeconds float64
}

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

		authAttempts, err := meter.Int64Counter(
			"clubhouse.auth.attempts",
			metric.WithDescription("Authentication attempts (login/register)"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		authFailures, err := meter.Int64Counter(
			"clubhouse.auth.failures",
			metric.WithDescription("Authentication failures by reason"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		authSessionsCreated, err := meter.Int64Counter(
			"clubhouse.auth.sessions.created",
			metric.WithDescription("Sessions created"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		authSessionsExpired, err := meter.Int64Counter(
			"clubhouse.auth.sessions.expired",
			metric.WithDescription("Sessions expired or invalidated"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		authTotpVerifications, err := meter.Int64Counter(
			"clubhouse.auth.totp.verifications",
			metric.WithDescription("TOTP verification attempts"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		authPasswordResets, err := meter.Int64Counter(
			"clubhouse.auth.password_resets",
			metric.WithDescription("Password reset operations"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		ratelimitViolations, err := meter.Int64Counter(
			"clubhouse.ratelimit.violations",
			metric.WithDescription("Rate limit violations"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		ratelimitLockouts, err := meter.Int64Counter(
			"clubhouse.ratelimit.lockouts",
			metric.WithDescription("Rate limit lockouts"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		ratelimitCacheKeys, err := meter.Int64Counter(
			"clubhouse.ratelimit.cache_keys",
			metric.WithDescription("Rate limit cache key creations"),
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

		reactionsAdded, err := meter.Int64Counter(
			"clubhouse.reactions.added",
			metric.WithDescription("Number of reactions added"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		reactionsRemoved, err := meter.Int64Counter(
			"clubhouse.reactions.removed",
			metric.WithDescription("Number of reactions removed"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		postsDeleted, err := meter.Int64Counter(
			"clubhouse.posts.deleted",
			metric.WithDescription("Number of posts deleted"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		postsRestored, err := meter.Int64Counter(
			"clubhouse.posts.restored",
			metric.WithDescription("Number of posts restored"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		commentsDeleted, err := meter.Int64Counter(
			"clubhouse.comments.deleted",
			metric.WithDescription("Number of comments deleted"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		commentsRestored, err := meter.Int64Counter(
			"clubhouse.comments.restored",
			metric.WithDescription("Number of comments restored"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		notificationsCreated, err := meter.Int64Counter(
			"clubhouse.notifications.created",
			metric.WithDescription("Number of notifications created"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		notificationsDelivered, err := meter.Int64Counter(
			"clubhouse.notifications.delivered",
			metric.WithDescription("Number of notifications delivered"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		notificationsFailed, err := meter.Int64Counter(
			"clubhouse.notifications.delivery_failed",
			metric.WithDescription("Number of notification delivery failures"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		linkMetadataFetchAttempts, err := meter.Int64Counter(
			"clubhouse.links.metadata.fetch.attempts",
			metric.WithDescription("Number of link metadata fetch attempts"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		linkMetadataFetchSuccess, err := meter.Int64Counter(
			"clubhouse.links.metadata.fetch.success",
			metric.WithDescription("Number of successful link metadata fetches"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		linkMetadataFetchFailures, err := meter.Int64Counter(
			"clubhouse.links.metadata.fetch.failures",
			metric.WithDescription("Number of failed link metadata fetches"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		linkMetadataFetchDuration, err := meter.Float64Histogram(
			"clubhouse.links.metadata.fetch.duration_ms",
			metric.WithDescription("Duration of link metadata fetches in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		searchQueries, err := meter.Int64Counter(
			"clubhouse.search.queries",
			metric.WithDescription("Number of search queries executed"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		searchResults, err := meter.Int64Histogram(
			"clubhouse.search.results",
			metric.WithDescription("Search result counts"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		searchDuration, err := meter.Float64Histogram(
			"clubhouse.search.duration_ms",
			metric.WithDescription("Search request duration in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		cacheHits, err := meter.Int64Counter(
			"clubhouse.cache.hits",
			metric.WithDescription("Number of cache hits"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		cacheMisses, err := meter.Int64Counter(
			"clubhouse.cache.misses",
			metric.WithDescription("Number of cache misses"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		uploadAttempts, err := meter.Int64Counter(
			"clubhouse.uploads.attempts",
			metric.WithDescription("Number of upload attempts"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		uploadSize, err := meter.Float64Histogram(
			"clubhouse.uploads.size_bytes",
			metric.WithDescription("Uploaded image sizes in bytes"),
			metric.WithUnit("By"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		adminActions, err := meter.Int64Counter(
			"clubhouse.admin.actions",
			metric.WithDescription("Number of admin actions performed"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		adminAuditLogViews, err := meter.Int64Counter(
			"clubhouse.admin.audit_log_views",
			metric.WithDescription("Number of audit log views"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		sectionsViews, err := meter.Int64Counter(
			"clubhouse.sections.views",
			metric.WithDescription("Section/feed views"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		postsUpdated, err := meter.Int64Counter(
			"clubhouse.posts.updated",
			metric.WithDescription("Number of posts updated"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		commentsUpdated, err := meter.Int64Counter(
			"clubhouse.comments.updated",
			metric.WithDescription("Number of comments updated"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		frontendWebVitals, err := meter.Float64Histogram(
			"clubhouse.frontend.web_vitals",
			metric.WithDescription("Frontend Web Vitals values"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		frontendApiDuration, err := meter.Float64Histogram(
			"clubhouse.frontend.api.request.duration_ms",
			metric.WithDescription("Frontend API request duration in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		frontendWebsocketDuration, err := meter.Float64Histogram(
			"clubhouse.frontend.websocket.connect.duration_ms",
			metric.WithDescription("Frontend WebSocket connect time in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		frontendAssetDuration, err := meter.Float64Histogram(
			"clubhouse.frontend.asset.load.duration_ms",
			metric.WithDescription("Frontend asset load duration in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		frontendComponentDuration, err := meter.Float64Histogram(
			"clubhouse.frontend.component.render.duration_ms",
			metric.WithDescription("Frontend component render duration in milliseconds"),
			metric.WithUnit("ms"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbConnectionsOpen, err := meter.Int64UpDownCounter(
			"clubhouse_db_connections_open",
			metric.WithDescription("Number of open database connections"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbConnectionsInUse, err := meter.Int64UpDownCounter(
			"clubhouse_db_connections_in_use",
			metric.WithDescription("Number of database connections currently in use"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbConnectionsIdle, err := meter.Int64UpDownCounter(
			"clubhouse_db_connections_idle",
			metric.WithDescription("Number of idle database connections in the pool"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbConnectionWaitCount, err := meter.Int64Counter(
			"clubhouse_db_connection_wait_count",
			metric.WithDescription("Total number of connections waited for"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbConnectionWaitDuration, err := meter.Float64Counter(
			"clubhouse_db_connection_wait_duration_seconds",
			metric.WithDescription("Total time blocked waiting for new connections in seconds"),
			metric.WithUnit("s"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbQueryErrors, err := meter.Int64Counter(
			"clubhouse_db_query_errors_total",
			metric.WithDescription("Total number of database query errors"),
		)
		if err != nil {
			metricsInitErr = err
			return
		}

		dbTransactions, err := meter.Int64Counter(
			"clubhouse_db_transactions_total",
			metric.WithDescription("Total number of database transactions"),
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
			authAttempts:              authAttempts,
			authFailures:              authFailures,
			authSessionsCreated:       authSessionsCreated,
			authSessionsExpired:       authSessionsExpired,
			authTotpVerifications:     authTotpVerifications,
			authPasswordResets:        authPasswordResets,
			ratelimitViolations:       ratelimitViolations,
			ratelimitLockouts:         ratelimitLockouts,
			ratelimitCacheKeys:        ratelimitCacheKeys,
			postsCreated:              postsCreated,
			commentsCreated:           commentsCreated,
			reactionsAdded:            reactionsAdded,
			reactionsRemoved:          reactionsRemoved,
			postsDeleted:              postsDeleted,
			postsRestored:             postsRestored,
			commentsDeleted:           commentsDeleted,
			commentsRestored:          commentsRestored,
			notificationsCreated:      notificationsCreated,
			notificationsDelivered:    notificationsDelivered,
			notificationsFailed:       notificationsFailed,
			linkMetadataFetchAttempts: linkMetadataFetchAttempts,
			linkMetadataFetchSuccess:  linkMetadataFetchSuccess,
			linkMetadataFetchFailures: linkMetadataFetchFailures,
			linkMetadataFetchDuration: linkMetadataFetchDuration,
			searchQueries:             searchQueries,
			searchResults:             searchResults,
			searchDuration:            searchDuration,
			cacheHits:                 cacheHits,
			cacheMisses:               cacheMisses,
			uploadAttempts:            uploadAttempts,
			uploadSize:                uploadSize,
			adminActions:              adminActions,
			adminAuditLogViews:        adminAuditLogViews,
			sectionsViews:             sectionsViews,
			postsUpdated:              postsUpdated,
			commentsUpdated:           commentsUpdated,
			frontendWebVitals:         frontendWebVitals,
			frontendApiDuration:       frontendApiDuration,
			frontendWebsocketDuration: frontendWebsocketDuration,
			frontendAssetDuration:     frontendAssetDuration,
			frontendComponentDuration: frontendComponentDuration,
			dbConnectionsOpen:         dbConnectionsOpen,
			dbConnectionsInUse:        dbConnectionsInUse,
			dbConnectionsIdle:         dbConnectionsIdle,
			dbConnectionWaitCount:     dbConnectionWaitCount,
			dbConnectionWaitDuration:  dbConnectionWaitDuration,
			dbQueryErrors:             dbQueryErrors,
			dbTransactions:            dbTransactions,
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

// RecordAuthAttempt increments the auth attempt counter.
func RecordAuthAttempt(ctx context.Context, attemptType string, result string) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("type", attemptType),
		attribute.String("result", result),
	}
	m.authAttempts.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordAuthFailure increments the auth failure counter.
func RecordAuthFailure(ctx context.Context, reason string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		return
	}
	m.authFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

// RecordAuthSessionCreated increments the session created counter.
func RecordAuthSessionCreated(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.authSessionsCreated.Add(ctx, 1)
}

// RecordAuthSessionExpired increments the session expired counter.
func RecordAuthSessionExpired(ctx context.Context, reason string, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(reason) != "" {
		attrs = append(attrs, attribute.String("reason", reason))
	}
	if len(attrs) == 0 {
		m.authSessionsExpired.Add(ctx, count)
		return
	}
	m.authSessionsExpired.Add(ctx, count, metric.WithAttributes(attrs...))
}

// UpdateDBStats records database connection pool statistics.
func UpdateDBStats(ctx context.Context, db *sql.DB) {
	m := getMetrics()
	if m == nil || db == nil {
		return
	}

	stats := db.Stats()
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()

	open := int64(stats.OpenConnections)
	inUse := int64(stats.InUse)
	idle := int64(stats.Idle)
	waitCount := int64(stats.WaitCount)
	waitSeconds := stats.WaitDuration.Seconds()

	if !dbStatsSnapshot.initialized {
		m.dbConnectionsOpen.Add(ctx, open)
		m.dbConnectionsInUse.Add(ctx, inUse)
		m.dbConnectionsIdle.Add(ctx, idle)
		if waitCount > 0 {
			m.dbConnectionWaitCount.Add(ctx, waitCount)
		}
		if waitSeconds > 0 {
			m.dbConnectionWaitDuration.Add(ctx, waitSeconds)
		}
		dbStatsSnapshot = dbStatsState{
			initialized: true,
			open:        open,
			inUse:       inUse,
			idle:        idle,
			waitCount:   waitCount,
			waitSeconds: waitSeconds,
		}
		return
	}

	m.dbConnectionsOpen.Add(ctx, open-dbStatsSnapshot.open)
	m.dbConnectionsInUse.Add(ctx, inUse-dbStatsSnapshot.inUse)
	m.dbConnectionsIdle.Add(ctx, idle-dbStatsSnapshot.idle)

	waitDelta := waitCount - dbStatsSnapshot.waitCount
	if waitDelta > 0 {
		m.dbConnectionWaitCount.Add(ctx, waitDelta)
	}
	waitSecondsDelta := waitSeconds - dbStatsSnapshot.waitSeconds
	if waitSecondsDelta > 0 {
		m.dbConnectionWaitDuration.Add(ctx, waitSecondsDelta)
	}

	dbStatsSnapshot.open = open
	dbStatsSnapshot.inUse = inUse
	dbStatsSnapshot.idle = idle
	dbStatsSnapshot.waitCount = waitCount
	dbStatsSnapshot.waitSeconds = waitSeconds
}

// StartDBStatsReporter periodically updates database stats until the context is done.
func StartDBStatsReporter(ctx context.Context, db *sql.DB, interval time.Duration) {
	if db == nil {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	UpdateDBStats(ctx, db)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			UpdateDBStats(ctx, db)
		}
	}
}

// RecordDBQueryError increments the database query error counter.
func RecordDBQueryError(ctx context.Context, queryType, errorType string) {
	m := getMetrics()
	if m == nil {
		return
	}

	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(queryType) != "" {
		attrs = append(attrs, attribute.String("query_type", queryType))
	}
	if strings.TrimSpace(errorType) != "" {
		attrs = append(attrs, attribute.String("error_type", errorType))
	}
	if len(attrs) == 0 {
		m.dbQueryErrors.Add(ctx, 1)
		return
	}
	m.dbQueryErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordDBTransaction increments the database transaction counter.
func RecordDBTransaction(ctx context.Context, status string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(status) == "" {
		return
	}
	m.dbTransactions.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
}

// RecordAuthTOTPVerification increments the TOTP verification counter.
func RecordAuthTOTPVerification(ctx context.Context, result string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(result) == "" {
		return
	}
	m.authTotpVerifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// RecordAuthPasswordReset increments the password reset counter.
func RecordAuthPasswordReset(ctx context.Context, stage string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(stage) == "" {
		return
	}
	m.authPasswordResets.Add(ctx, 1, metric.WithAttributes(attribute.String("stage", stage)))
}

// RecordRateLimitViolation increments the rate limit violation counter.
func RecordRateLimitViolation(ctx context.Context, limitType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(limitType) == "" {
		return
	}
	m.ratelimitViolations.Add(ctx, 1, metric.WithAttributes(attribute.String("limit_type", limitType)))
}

// RecordRateLimitCacheKey increments the rate limit cache key counter.
func RecordRateLimitCacheKey(ctx context.Context, limitType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(limitType) == "" {
		return
	}
	m.ratelimitCacheKeys.Add(ctx, 1, metric.WithAttributes(attribute.String("limit_type", limitType)))
}

// RecordRateLimitLockout increments the rate limit lockout counter.
func RecordRateLimitLockout(ctx context.Context, reason string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		return
	}
	m.ratelimitLockouts.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
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

// RecordReactionAdded increments the reaction added counter.
func RecordReactionAdded(ctx context.Context, target string) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.reactionsAdded.Add(ctx, 1, metric.WithAttributes(attribute.String("target", target)))
}

// RecordReactionRemoved increments the reaction removed counter.
func RecordReactionRemoved(ctx context.Context, target string) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.reactionsRemoved.Add(ctx, 1, metric.WithAttributes(attribute.String("target", target)))
}

// RecordPostDeleted increments the post deleted counter.
func RecordPostDeleted(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.postsDeleted.Add(ctx, 1)
}

// RecordPostRestored increments the post restored counter.
func RecordPostRestored(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.postsRestored.Add(ctx, 1)
}

// RecordCommentDeleted increments the comment deleted counter.
func RecordCommentDeleted(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.commentsDeleted.Add(ctx, 1)
}

// RecordCommentRestored increments the comment restored counter.
func RecordCommentRestored(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.commentsRestored.Add(ctx, 1)
}

// RecordNotificationsCreated increments the notification created counter.
func RecordNotificationsCreated(ctx context.Context, notificationType string, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	m.notificationsCreated.Add(ctx, count, metric.WithAttributes(attribute.String("type", notificationType)))
}

// RecordNotificationDelivered increments the notification delivered counter.
func RecordNotificationDelivered(ctx context.Context, channel string, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	m.notificationsDelivered.Add(ctx, count, metric.WithAttributes(attribute.String("channel", channel)))
}

// RecordNotificationDeliveryFailed increments the notification delivery failure counter.
func RecordNotificationDeliveryFailed(ctx context.Context, channel string, errorType string, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("channel", channel),
	}
	if strings.TrimSpace(errorType) != "" {
		attrs = append(attrs, attribute.String("error_type", errorType))
	}
	m.notificationsFailed.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordLinkMetadataFetchAttempt increments the link metadata fetch attempt counter.
func RecordLinkMetadataFetchAttempt(ctx context.Context, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	m.linkMetadataFetchAttempts.Add(ctx, count)
}

// RecordLinkMetadataFetchSuccess increments the link metadata fetch success counter.
func RecordLinkMetadataFetchSuccess(ctx context.Context, count int64) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	m.linkMetadataFetchSuccess.Add(ctx, count)
}

// RecordLinkMetadataFetchFailure increments the link metadata fetch failure counter.
func RecordLinkMetadataFetchFailure(ctx context.Context, count int64, domain string, errorType string) {
	if count <= 0 {
		return
	}
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(domain) != "" {
		attrs = append(attrs, attribute.String("domain", domain))
	}
	if strings.TrimSpace(errorType) != "" {
		attrs = append(attrs, attribute.String("error_type", errorType))
	}
	if len(attrs) == 0 {
		m.linkMetadataFetchFailures.Add(ctx, count)
		return
	}
	m.linkMetadataFetchFailures.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordLinkMetadataFetchDuration records how long link metadata fetches take.
func RecordLinkMetadataFetchDuration(ctx context.Context, duration time.Duration) {
	m := getMetrics()
	if m == nil {
		return
	}
	if duration < 0 {
		return
	}
	m.linkMetadataFetchDuration.Record(ctx, float64(duration.Milliseconds()))
}

// RecordSearchQuery records a completed search query.
func RecordSearchQuery(ctx context.Context, scope string, resultCount int, duration time.Duration) {
	m := getMetrics()
	if m == nil {
		return
	}
	if duration < 0 {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(scope) != "" {
		attrs = append(attrs, attribute.String("scope", scope))
	}
	m.searchQueries.Add(ctx, 1, metric.WithAttributes(attrs...))
	if resultCount >= 0 {
		m.searchResults.Record(ctx, int64(resultCount), metric.WithAttributes(attrs...))
	}
	m.searchDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// RecordCacheHit records a cache hit.
func RecordCacheHit(ctx context.Context, cacheType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(cacheType) != "" {
		attrs = append(attrs, attribute.String("cache_type", cacheType))
	}
	if len(attrs) == 0 {
		m.cacheHits.Add(ctx, 1)
		return
	}
	m.cacheHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordCacheMiss records a cache miss.
func RecordCacheMiss(ctx context.Context, cacheType string) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(cacheType) != "" {
		attrs = append(attrs, attribute.String("cache_type", cacheType))
	}
	if len(attrs) == 0 {
		m.cacheMisses.Add(ctx, 1)
		return
	}
	m.cacheMisses.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordUploadAttempt records an upload attempt.
func RecordUploadAttempt(ctx context.Context, result string, fileType string, sizeBytes int64) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if strings.TrimSpace(result) != "" {
		attrs = append(attrs, attribute.String("result", result))
	}
	if strings.TrimSpace(fileType) != "" {
		attrs = append(attrs, attribute.String("filetype", fileType))
	}
	m.uploadAttempts.Add(ctx, 1, metric.WithAttributes(attrs...))
	if sizeBytes > 0 {
		m.uploadSize.Record(ctx, float64(sizeBytes), metric.WithAttributes(attrs...))
	}
}

// RecordAdminAction records a completed admin action.
func RecordAdminAction(ctx context.Context, action string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(action) == "" {
		return
	}
	m.adminActions.Add(ctx, 1, metric.WithAttributes(attribute.String("action", action)))
}

// RecordAdminAuditLogView records an audit log view.
func RecordAdminAuditLogView(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.adminAuditLogViews.Add(ctx, 1)
}

// RecordSectionView records a section view.
func RecordSectionView(ctx context.Context, sectionID string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if strings.TrimSpace(sectionID) == "" {
		return
	}
	m.sectionsViews.Add(ctx, 1, metric.WithAttributes(attribute.String("section_id", sectionID)))
}

// RecordPostUpdated records a post update.
func RecordPostUpdated(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.postsUpdated.Add(ctx, 1)
}

// RecordCommentUpdated records a comment update.
func RecordCommentUpdated(ctx context.Context) {
	m := getMetrics()
	if m == nil {
		return
	}
	m.commentsUpdated.Add(ctx, 1)
}

// RecordFrontendWebVital records a Web Vital metric from the frontend.
func RecordFrontendWebVital(ctx context.Context, name string, value float64, rating string, navigationType string, unit string) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("name", name),
	}
	if rating != "" {
		attrs = append(attrs, attribute.String("rating", rating))
	}
	if navigationType != "" {
		attrs = append(attrs, attribute.String("navigation_type", navigationType))
	}
	if unit != "" {
		attrs = append(attrs, attribute.String("unit", unit))
	}
	m.frontendWebVitals.Record(ctx, value, metric.WithAttributes(attrs...))
}

// RecordFrontendAPIDuration records the duration of frontend API calls.
func RecordFrontendAPIDuration(ctx context.Context, endpoint string, method string, status int, durationMs float64) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("endpoint", endpoint),
		attribute.String("method", method),
		attribute.Int("status", status),
	}
	m.frontendApiDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordFrontendWebsocketConnect records the WebSocket connection time.
func RecordFrontendWebsocketConnect(ctx context.Context, outcome string, durationMs float64) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if outcome != "" {
		attrs = append(attrs, attribute.String("outcome", outcome))
	}
	m.frontendWebsocketDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordFrontendAssetLoad records asset load durations.
func RecordFrontendAssetLoad(ctx context.Context, resourceType string, name string, durationMs float64) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if resourceType != "" {
		attrs = append(attrs, attribute.String("resource_type", resourceType))
	}
	if name != "" {
		attrs = append(attrs, attribute.String("name", name))
	}
	m.frontendAssetDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordFrontendComponentRender records component render durations.
func RecordFrontendComponentRender(ctx context.Context, component string, durationMs float64) {
	m := getMetrics()
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{}
	if component != "" {
		attrs = append(attrs, attribute.String("component", component))
	}
	m.frontendComponentDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}
