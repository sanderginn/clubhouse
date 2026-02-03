package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/sanderginn/clubhouse/internal/observability"
	"github.com/sanderginn/clubhouse/internal/services"
	"github.com/sanderginn/clubhouse/internal/testutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRequireAuthRecordsMissingSessionMetric(t *testing.T) {
	reader, ctx := setupAuthFailureMetrics(t)
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	handler := RequireAuth(redisClient, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected auth middleware to block request")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/private", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if got := getAuthFailureCount(t, reader, ctx, "missing_session"); got != 1 {
		t.Fatalf("expected missing_session count 1, got %d", got)
	}
}

func TestRequireAuthRecordsInvalidSessionMetric(t *testing.T) {
	reader, ctx := setupAuthFailureMetrics(t)
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	handler := RequireAuth(redisClient, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected auth middleware to block request")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/private", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "missing"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if got := getAuthFailureCount(t, reader, ctx, "invalid_session"); got != 1 {
		t.Fatalf("expected invalid_session count 1, got %d", got)
	}
}

func TestRequireAdminRecordsInvalidSessionMetric(t *testing.T) {
	reader, ctx := setupAuthFailureMetrics(t)
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	handler := RequireAdmin(redisClient, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected auth middleware to block request")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "missing"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if got := getAuthFailureCount(t, reader, ctx, "invalid_session"); got != 1 {
		t.Fatalf("expected invalid_session count 1, got %d", got)
	}
}

func TestRequireAuthRecordsSuspendedMetric(t *testing.T) {
	if os.Getenv("CLUBHOUSE_TEST_DATABASE_URL") == "" {
		t.Skip("CLUBHOUSE_TEST_DATABASE_URL not set")
	}

	reader, ctx := setupAuthFailureMetrics(t)
	db := testutil.GetTestDB(t)
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupTables(t, db)
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	userID := uuid.New()
	userSuffix := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO users (id, username, email, password_hash, approved_at, suspended_at, created_at)
		VALUES ($1, $2, $3, $4, now(), now(), now())
	`, userID, "suspended_user_"+userSuffix, "suspended_"+userSuffix+"@example.com", "hashed")
	if err != nil {
		t.Fatalf("failed to insert suspended user: %v", err)
	}

	sessionService := services.NewSessionService(redisClient)
	session, err := sessionService.CreateSession(ctx, userID, "suspended_user", false)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	handler := RequireAuth(redisClient, db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected auth middleware to block suspended user")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/private", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	if got := getAuthFailureCount(t, reader, ctx, "suspended"); got != 1 {
		t.Fatalf("expected suspended count 1, got %d", got)
	}
}

func setupAuthFailureMetrics(t *testing.T) (*sdkmetric.ManualReader, context.Context) {
	t.Helper()

	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previousProvider)
	})

	observability.ResetMetricsForTest()
	if err := observability.InitMetrics(); err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	return reader, ctx
}

func getAuthFailureCount(t *testing.T, reader *sdkmetric.ManualReader, ctx context.Context, reason string) int64 {
	t.Helper()

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	for _, scopeMetrics := range metrics.ScopeMetrics {
		for _, metricItem := range scopeMetrics.Metrics {
			if metricItem.Name != "clubhouse.auth.failures" {
				continue
			}
			sum, ok := metricItem.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric clubhouse.auth.failures has unexpected data type %T", metricItem.Data)
			}
			for _, dataPoint := range sum.DataPoints {
				if attributeMatchesReason(dataPoint.Attributes, reason) {
					return dataPoint.Value
				}
			}
		}
	}

	t.Fatalf("metric clubhouse.auth.failures with reason %s not found", reason)
	return 0
}

func attributeMatchesReason(set attribute.Set, reason string) bool {
	for _, kv := range set.ToSlice() {
		if string(kv.Key) == "reason" && kv.Value.AsString() == reason {
			return true
		}
	}
	return false
}
