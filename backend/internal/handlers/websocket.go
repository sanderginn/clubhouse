package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/sanderginn/clubhouse/internal/middleware"
	"github.com/sanderginn/clubhouse/internal/observability"
)

const (
	wsReadLimit   = 64 * 1024
	wsPongWait    = 60 * time.Second
	wsPingPeriod  = 50 * time.Second
	wsWriteWait   = 10 * time.Second
	wsSubscribe   = "subscribe"
	wsUnsubscribe = "unsubscribe"
	wsPing        = "ping"
	userMentions  = "user:%s:mentions"
	userNotify    = "user:%s:notifications"
	sectionPrefix = "section:%s"
)

// WebSocket spans:
// - websocket.message.receive (attrs: user_id, message_type)
// - websocket.message.send (attrs: user_id, message_type, channel)
const (
	wsSpanMessageReceive = "websocket.message.receive"
	wsSpanMessageSend    = "websocket.message.send"
)

type wsConnection struct {
	conn          *websocket.Conn
	pubsub        *redis.PubSub
	subscriptions map[string]struct{}
	writeMu       sync.Mutex
	cancel        context.CancelFunc
	userID        uuid.UUID
}

// WebSocketHandler manages WebSocket connections.
type WebSocketHandler struct {
	mu          sync.RWMutex
	connections map[uuid.UUID]*wsConnection
	redis       *redis.Client
	upgrader    websocket.Upgrader
}

// NewWebSocketHandler creates a WebSocket handler with connection tracking.
func NewWebSocketHandler(redis *redis.Client) *WebSocketHandler {
	return &WebSocketHandler{
		connections: make(map[uuid.UUID]*wsConnection),
		redis:       redis,
		upgrader: websocket.Upgrader{
			CheckOrigin: sameOrigin,
		},
	}
}

// HandleWS upgrades authenticated requests to WebSocket connections.
func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	observability.LogInfo(ctx, "WebSocket connection attempt",
		"method", r.Method,
		"origin", r.Header.Get("Origin"),
		"host", r.Host)

	if r.Method != http.MethodGet {
		observability.LogInfo(ctx, "WebSocket rejected: non-GET method", "method", r.Method)
		writeError(r.Context(), w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		observability.LogInfo(ctx, "WebSocket auth failed", "error", err.Error())
		writeError(r.Context(), w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	observability.LogInfo(ctx, "Upgrading WebSocket connection", "user_id", userID.String())
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		observability.LogInfo(ctx, "WebSocket upgrade failed", "error", err.Error())
		return
	}
	observability.LogInfo(ctx, "WebSocket connection established")

	ctx, cancel := context.WithCancel(r.Context())
	wsConn := &wsConnection{
		conn:          conn,
		subscriptions: make(map[string]struct{}),
		cancel:        cancel,
		userID:        userID,
	}

	h.registerConnection(r.Context(), userID, wsConn)
	defer h.unregisterConnection(r.Context(), userID, wsConn)

	conn.SetReadLimit(wsReadLimit)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	wsConn.pubsub = h.redis.Subscribe(ctx)
	h.subscribeChannels(ctx, wsConn, []string{
		formatChannel(userMentions, userID),
		formatChannel(userNotify, userID),
	})

	go h.writeLoop(ctx, wsConn)
	h.readLoop(ctx, wsConn)
}

func (h *WebSocketHandler) registerConnection(ctx context.Context, userID uuid.UUID, wsConn *wsConnection) {
	h.mu.Lock()
	if existing := h.connections[userID]; existing != nil {
		// One active connection per user; latest connection wins.
		h.closeConnection(existing)
	}
	h.connections[userID] = wsConn
	h.mu.Unlock()

	h.addEvent(ctx, userID, "websocket_connected")
	observability.RecordWebsocketConnect(ctx)
}

func (h *WebSocketHandler) unregisterConnection(ctx context.Context, userID uuid.UUID, wsConn *wsConnection) {
	h.mu.Lock()
	if existing := h.connections[userID]; existing == wsConn {
		delete(h.connections, userID)
	}
	h.mu.Unlock()

	h.closeConnection(wsConn)
	h.addEvent(ctx, userID, "websocket_disconnected")
	observability.RecordWebsocketDisconnect(ctx)
}

func (h *WebSocketHandler) closeConnection(wsConn *wsConnection) {
	if wsConn == nil {
		return
	}
	wsConn.cancel()
	if wsConn.pubsub != nil {
		_ = wsConn.pubsub.Close()
	}
	wsConn.writeMu.Lock()
	_ = wsConn.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = wsConn.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	wsConn.writeMu.Unlock()
	_ = wsConn.conn.Close()
}

func (h *WebSocketHandler) readLoop(ctx context.Context, wsConn *wsConnection) {
	for {
		_, payload, err := wsConn.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			spanCtx, span := h.startMessageSpan(ctx, wsConn, wsSpanMessageReceive, "invalid")
			span.RecordError(err)
			span.End()
			observability.RecordWebsocketMessageReceived(spanCtx, "invalid")
			observability.RecordWebsocketError(spanCtx, "invalid_message", "invalid")
			continue
		}

		messageType := msg.Type
		if messageType == "" {
			messageType = "unknown"
		}

		spanCtx, span := h.startMessageSpan(ctx, wsConn, wsSpanMessageReceive, messageType)
		observability.RecordWebsocketMessageReceived(spanCtx, messageType)
		switch msg.Type {
		case wsSubscribe:
			var data subscribePayload
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				span.RecordError(err)
				observability.RecordWebsocketError(spanCtx, "invalid_payload", messageType)
				span.End()
				continue
			}
			h.addSubscriptions(spanCtx, wsConn, data.SectionIDs, messageType)
		case wsUnsubscribe:
			var data subscribePayload
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				span.RecordError(err)
				observability.RecordWebsocketError(spanCtx, "invalid_payload", messageType)
				span.End()
				continue
			}
			h.removeSubscriptions(spanCtx, wsConn, data.SectionIDs, messageType)
		case wsPing:
			// Ping messages are no-ops but still traced/metriced.
		default:
			// Custom event types are traced and counted; no handler yet.
		}
		span.End()
	}
}

func (h *WebSocketHandler) writeLoop(ctx context.Context, wsConn *wsConnection) {
	go h.pingLoop(ctx, wsConn)

	for {
		msg, err := wsConn.pubsub.ReceiveMessage(ctx)
		if err != nil {
			wsConn.cancel()
			return
		}

		payload := []byte(msg.Payload)
		messageType := "message"
		if json.Valid(payload) {
			var event wsEvent
			if err := json.Unmarshal(payload, &event); err == nil && event.Type != "" {
				messageType = event.Type
			} else if err != nil {
				messageType = "unknown"
			}
		}
		if !json.Valid(payload) {
			payload = h.wrapPayload(msg.Payload)
			messageType = "message"
		}

		spanCtx, span := h.startMessageSpan(ctx, wsConn, wsSpanMessageSend, messageType)
		span.SetAttributes(attribute.String("channel", msg.Channel))
		observability.RecordWebsocketMessageSent(spanCtx, messageType)
		h.sendMessage(wsConn, payload)
		span.End()
	}
}

func (h *WebSocketHandler) pingLoop(ctx context.Context, wsConn *wsConnection) {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case <-pingTicker.C:
			h.sendPing(wsConn)
		case <-ctx.Done():
			return
		}
	}
}

func (h *WebSocketHandler) wrapPayload(payload string) []byte {
	event := wsEvent{
		Type:      "message",
		Data:      map[string]string{"payload": payload},
		Timestamp: time.Now().UTC(),
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return []byte(`{"type":"message","data":{"payload":"invalid"},"timestamp":"0001-01-01T00:00:00Z"}`)
	}
	return bytes
}

func (h *WebSocketHandler) sendMessage(wsConn *wsConnection, payload []byte) {
	wsConn.writeMu.Lock()
	defer wsConn.writeMu.Unlock()
	_ = wsConn.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = wsConn.conn.WriteMessage(websocket.TextMessage, payload)
}

func (h *WebSocketHandler) sendPing(wsConn *wsConnection) {
	wsConn.writeMu.Lock()
	defer wsConn.writeMu.Unlock()
	_ = wsConn.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = wsConn.conn.WriteMessage(websocket.PingMessage, nil)
}

func (h *WebSocketHandler) subscribeChannels(ctx context.Context, wsConn *wsConnection, channels []string) {
	if len(channels) == 0 {
		return
	}
	_ = wsConn.pubsub.Subscribe(ctx, channels...)
	for _, ch := range channels {
		wsConn.subscriptions[ch] = struct{}{}
	}
}

func (h *WebSocketHandler) addSubscriptions(ctx context.Context, wsConn *wsConnection, sectionIDs []string, messageType string) {
	channels := sectionChannels(sectionIDs)
	if len(channels) == 0 {
		return
	}

	var toSubscribe []string
	for _, ch := range channels {
		if _, ok := wsConn.subscriptions[ch]; !ok {
			toSubscribe = append(toSubscribe, ch)
		}
	}
	if len(toSubscribe) == 0 {
		return
	}

	_ = wsConn.pubsub.Subscribe(ctx, toSubscribe...)
	for _, ch := range toSubscribe {
		wsConn.subscriptions[ch] = struct{}{}
	}
	observability.RecordWebsocketSubscriptionAdded(ctx, messageType, len(toSubscribe))
}

func (h *WebSocketHandler) removeSubscriptions(ctx context.Context, wsConn *wsConnection, sectionIDs []string, messageType string) {
	channels := sectionChannels(sectionIDs)
	if len(channels) == 0 {
		return
	}

	var toUnsubscribe []string
	for _, ch := range channels {
		if strings.HasPrefix(ch, "section:") {
			if _, ok := wsConn.subscriptions[ch]; ok {
				toUnsubscribe = append(toUnsubscribe, ch)
			}
		}
	}
	if len(toUnsubscribe) == 0 {
		return
	}

	_ = wsConn.pubsub.Unsubscribe(ctx, toUnsubscribe...)
	for _, ch := range toUnsubscribe {
		delete(wsConn.subscriptions, ch)
	}
	observability.RecordWebsocketSubscriptionRemoved(ctx, messageType, len(toUnsubscribe))
}

func (h *WebSocketHandler) addEvent(ctx context.Context, userID uuid.UUID, event string) {
	tracer := otel.Tracer("clubhouse.websocket")
	_, span := tracer.Start(ctx, event)
	span.SetAttributes(attribute.String("user_id", userID.String()))
	span.End()
}

func (h *WebSocketHandler) startMessageSpan(ctx context.Context, wsConn *wsConnection, spanName, messageType string) (context.Context, trace.Span) {
	tracer := otel.Tracer("clubhouse.websocket")
	spanCtx, span := tracer.Start(ctx, spanName)
	span.SetAttributes(
		attribute.String("user_id", wsConn.userID.String()),
		attribute.String("message_type", messageType),
	)
	return spanCtx, span
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	ctx := r.Context()

	observability.LogInfo(ctx, "WebSocket origin check",
		"origin", origin,
		"host", r.Host)

	if origin == "" {
		observability.LogInfo(ctx, "WebSocket origin check failed: empty origin")
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		observability.LogInfo(ctx, "WebSocket origin check failed: parse error", "error", err.Error())
		return false
	}

	// Allow same origin
	if strings.EqualFold(u.Host, r.Host) {
		observability.LogInfo(ctx, "WebSocket origin check passed: same origin")
		return true
	}

	// Allow localhost:5173 in development (Vite dev server)
	// This covers both local development and Docker Compose frontend
	if strings.HasPrefix(u.Host, "localhost:5173") || u.Host == "127.0.0.1:5173" {
		observability.LogInfo(ctx, "WebSocket origin check passed: localhost:5173")
		return true
	}

	// Allow any origin in development when Host is backend:8080 (Docker Compose)
	// This is safe because in production, the backend won't be accessible at "backend:8080"
	if strings.Contains(r.Host, "backend:8080") || strings.Contains(r.Host, "localhost:8080") || strings.Contains(r.Host, "127.0.0.1:8080") {
		observability.LogInfo(ctx, "WebSocket origin check passed: development backend host")
		return true
	}

	observability.LogInfo(ctx, "WebSocket origin check failed: origin not allowed", "origin_host", u.Host)
	return false
}

type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type subscribePayload struct {
	SectionIDs []string `json:"sectionIds"`
}

func formatChannel(format string, id any) string {
	return fmt.Sprintf(format, id)
}

func sectionChannels(sectionIDs []string) []string {
	if len(sectionIDs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(sectionIDs))
	var channels []string
	for _, id := range sectionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		channel := formatChannel(sectionPrefix, id)
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		channels = append(channels, channel)
	}
	return channels
}
