package service

import (
	"log"
	"net/http"
	"time"

	"ICPCRemoteControl/internal/biz"

	"github.com/gorilla/websocket"
)

var (
	wsWriteWait = 10 * time.Second

	upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

// AdminWSHandler handles WebSocket connections from admin browser UIs.
type AdminWSHandler struct {
	hub *biz.Hub
}

// NewAdminWSHandler creates a new AdminWSHandler.
func NewAdminWSHandler(hub *biz.Hub) *AdminWSHandler {
	return &AdminWSHandler{hub: hub}
}

// Serve handles the admin WebSocket connection lifecycle.
func (h *AdminWSHandler) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[admin-ws] upgrade error: %v", err)
		return
	}

	admin := &biz.AdminConn{
		Conn: conn,
		Send: make(chan []byte, 128),
		Hub:  h.hub,
	}
	h.hub.RegisterAdmin(admin)

	defer func() {
		h.hub.UnregisterAdmin(admin)
		conn.Close()
	}()

	// Write pump: forward events from the Send channel to the WebSocket.
	go func() {
		defer func() {
			h.hub.UnregisterAdmin(admin)
		}()
		for msg := range admin.Send {
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[admin-ws] write error: %v", err)
				return
			}
		}
	}()

	// Read pump: just keep the connection alive; ignore incoming messages.
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
