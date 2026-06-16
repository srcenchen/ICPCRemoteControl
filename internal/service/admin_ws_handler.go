package service

import (
	"log"
	"net/http"
	"time"

	"ICPCRemoteControl/internal/biz"

	"github.com/gorilla/websocket"
)

var (
	wsWriteWait  = 10 * time.Second
	wsPingPeriod = 30 * time.Second
	wsPongWait   = 60 * time.Second

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

	// Ping/pong to keep connection alive through proxies.
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Write pump: forward events from the Send channel to the WebSocket.
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		pingTicker := time.NewTicker(wsPingPeriod)
		defer pingTicker.Stop()
		for {
			select {
			case msg, ok := <-admin.Send:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Printf("[admin-ws] write error: %v", err)
					return
				}
			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("[admin-ws] ping error: %v", err)
					return
				}
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
	<-writeDone
}
