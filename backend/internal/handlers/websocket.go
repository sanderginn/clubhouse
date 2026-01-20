package handlers

import (
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/net/websocket"

	"github.com/sanderginn/clubhouse/internal/middleware"
)

// WebSocketHandler manages WebSocket connections.
type WebSocketHandler struct {
	mu          sync.RWMutex
	connections map[uuid.UUID]*websocket.Conn
}

// NewWebSocketHandler creates a WebSocket handler with connection tracking.
func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		connections: make(map[uuid.UUID]*websocket.Conn),
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

	wsHandler := websocket.Handler(func(conn *websocket.Conn) {
		h.registerConnection(userID, conn)
		defer h.unregisterConnection(userID, conn)
		h.readLoop(conn)
	})

	wsHandler.ServeHTTP(w, r)
}

func (h *WebSocketHandler) registerConnection(userID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	if existing := h.connections[userID]; existing != nil && existing != conn {
		_ = existing.Close()
	}
	h.connections[userID] = conn
	h.mu.Unlock()

	log.Printf("websocket connected user_id=%s remote_addr=%s", userID, conn.RemoteAddr())
}

func (h *WebSocketHandler) unregisterConnection(userID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	if existing := h.connections[userID]; existing == conn {
		delete(h.connections, userID)
	}
	h.mu.Unlock()

	_ = conn.Close()
	log.Printf("websocket disconnected user_id=%s remote_addr=%s", userID, conn.RemoteAddr())
}

func (h *WebSocketHandler) readLoop(conn *websocket.Conn) {
	for {
		var payload []byte
		if err := websocket.Message.Receive(conn, &payload); err != nil {
			return
		}
	}
}
