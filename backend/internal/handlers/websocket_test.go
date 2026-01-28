package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/sanderginn/clubhouse/internal/testutil"
)

func TestSameOriginAllowlist(t *testing.T) {
	t.Setenv("WS_ORIGIN_ALLOWLIST", "https://example.com, foo.test:8443, http://admin.internal:9000")

	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{
			name:   "allowed origin in allowlist",
			host:   "api.local",
			origin: "https://example.com",
			want:   true,
		},
		{
			name:   "allowed origin with port",
			host:   "api.local",
			origin: "http://foo.test:8443",
			want:   true,
		},
		{
			name:   "denied origin not in allowlist",
			host:   "api.local",
			origin: "https://evil.example.com",
			want:   false,
		},
		{
			name:   "empty origin",
			host:   "api.local",
			origin: "",
			want:   false,
		},
		{
			name:   "malformed origin",
			host:   "api.local",
			origin: "://bad",
			want:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+test.host+"/api/v1/ws", nil)
			req.Host = test.host
			req.Header.Set("Origin", test.origin)

			if got := sameOrigin(req); got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestSameOriginDefaultAllowlist(t *testing.T) {
	t.Setenv("WS_ORIGIN_ALLOWLIST", "")

	req := httptest.NewRequest(http.MethodGet, "http://api.local/api/v1/ws", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	if got := sameOrigin(req); !got {
		t.Fatalf("expected default allowlist to allow localhost:5173")
	}
}

func TestWebSocketSubscribeDispatchAndUnsubscribe(t *testing.T) {
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	handler := NewWebSocketHandler(redisClient)
	userID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(createTestUserContext(r.Context(), userID, "wsuser", false))
		handler.HandleWS(w, r)
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	origin := server.URL
	t.Setenv("WS_ORIGIN_ALLOWLIST", origin)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	sectionID := "section-123"
	subscribe := mustMarshal(t, wsMessage{
		Type: wsSubscribe,
		Data: mustMarshal(t, subscribePayload{SectionIDs: []string{sectionID}}),
	})
	if err := conn.WriteMessage(websocket.TextMessage, subscribe); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}
	waitForSubscription(t, redisClient, formatChannel(sectionPrefix, sectionID), 1)

	event := wsEvent{
		Type:      "post_created",
		Data:      map[string]string{"id": "post-1"},
		Timestamp: time.Now().UTC(),
	}
	eventBytes := mustMarshal(t, event)
	if err := redisClient.Publish(context.Background(), formatChannel(sectionPrefix, sectionID), eventBytes).Err(); err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var got wsEvent
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if got.Type != event.Type {
		t.Fatalf("expected event type %q, got %q", event.Type, got.Type)
	}

	unsubscribe := mustMarshal(t, wsMessage{
		Type: wsUnsubscribe,
		Data: mustMarshal(t, subscribePayload{SectionIDs: []string{sectionID}}),
	})
	if err := conn.WriteMessage(websocket.TextMessage, unsubscribe); err != nil {
		t.Fatalf("failed to send unsubscribe: %v", err)
	}
	waitForSubscription(t, redisClient, formatChannel(sectionPrefix, sectionID), 0)

	time.Sleep(50 * time.Millisecond)
	if err := redisClient.Publish(context.Background(), formatChannel(sectionPrefix, sectionID), eventBytes).Err(); err != nil {
		t.Fatalf("failed to publish event after unsubscribe: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected timeout after unsubscribe, got message")
	}

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected timeout after unsubscribe, got %v", err)
	}
}

func TestWebSocketSubscribeDispatchSnakeCase(t *testing.T) {
	redisClient := testutil.GetTestRedis(t)
	t.Cleanup(func() {
		testutil.CleanupRedis(t)
		_ = redisClient.Close()
	})

	handler := NewWebSocketHandler(redisClient)
	userID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(createTestUserContext(r.Context(), userID, "wsuser", false))
		handler.HandleWS(w, r)
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	origin := server.URL
	t.Setenv("WS_ORIGIN_ALLOWLIST", origin)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	sectionID := "section-456"
	subscribe := mustMarshal(t, wsMessage{
		Type: wsSubscribe,
		Data: mustMarshal(t, map[string][]string{"section_ids": {sectionID}}),
	})
	if err := conn.WriteMessage(websocket.TextMessage, subscribe); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}
	waitForSubscription(t, redisClient, formatChannel(sectionPrefix, sectionID), 1)

	event := wsEvent{
		Type:      "comment_added",
		Data:      map[string]string{"id": "comment-1"},
		Timestamp: time.Now().UTC(),
	}
	eventBytes := mustMarshal(t, event)
	if err := redisClient.Publish(context.Background(), formatChannel(sectionPrefix, sectionID), eventBytes).Err(); err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var got wsEvent
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if got.Type != event.Type {
		t.Fatalf("expected event type %q, got %q", event.Type, got.Type)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	bytes, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return bytes
}

func waitForSubscription(t *testing.T, redisClient *redis.Client, channel string, expected int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for {
		counts, err := redisClient.PubSubNumSub(ctx, channel).Result()
		if err == nil {
			if count, ok := counts[channel]; ok && count == expected {
				return
			}
		}

		if ctx.Err() != nil {
			t.Fatalf("subscription count for %s did not reach %d", channel, expected)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
