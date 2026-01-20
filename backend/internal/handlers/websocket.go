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

	"github.com/sanderginn/clubhouse/internal/middleware"
)

const (
	wsReadLimit   = 64 * 1024
	wsPongWait    = 60 * time.Second
	wsPingPeriod  = 50 * time.Second
	wsWriteWait   = 10 * time.Second
	wsSubscribe   = "subscribe"
	wsUnsubscribe = "unsubscribe"
	userMentions  = "user:%s:mentions"
	userNotify    = "user:%s:notifications"
	sectionPrefix = "section:%s"
)

type wsConnection struct {
	conn          *websocket.Conn
	pubsub        *redis.PubSub
	subscriptions map[string]struct{}
	writeMu       sync.Mutex
	cancel        context.CancelFunc
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
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET requests are allowed")
		return
	}

	userID, err := middleware.GetUserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid user ID")
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	wsConn := &wsConnection{
		conn:          conn,
		subscriptions: make(map[string]struct{}),
		cancel:        cancel,
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
}

func (h *WebSocketHandler) unregisterConnection(ctx context.Context, userID uuid.UUID, wsConn *wsConnection) {
	h.mu.Lock()
	if existing := h.connections[userID]; existing == wsConn {
		delete(h.connections, userID)
	}
	h.mu.Unlock()

	h.closeConnection(wsConn)
	h.addEvent(ctx, userID, "websocket_disconnected")
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
			continue
		}
		switch msg.Type {
		case wsSubscribe:
			var data subscribePayload
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				continue
			}
			h.addSubscriptions(ctx, wsConn, data.SectionIDs)
		case wsUnsubscribe:
			var data subscribePayload
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				continue
			}
			h.removeSubscriptions(ctx, wsConn, data.SectionIDs)
		default:
			continue
		}
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
		if !json.Valid(payload) {
			payload = h.wrapPayload(msg.Payload)
		}
		h.sendMessage(wsConn, payload)
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

func (h *WebSocketHandler) addSubscriptions(ctx context.Context, wsConn *wsConnection, sectionIDs []string) {
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
}

func (h *WebSocketHandler) removeSubscriptions(ctx context.Context, wsConn *wsConnection, sectionIDs []string) {
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
}

func (h *WebSocketHandler) addEvent(ctx context.Context, userID uuid.UUID, event string) {
	tracer := otel.Tracer("clubhouse.websocket")
	_, span := tracer.Start(ctx, event)
	span.SetAttributes(attribute.String("user_id", userID.String()))
	span.End()
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
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
