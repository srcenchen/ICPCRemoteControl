package biz

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"ICPCRemoteControl/internal/data"
	"ICPCRemoteControl/internal/model"

	"github.com/gorilla/websocket"
)

// ClientConn wraps a TCP connection from a contestant machine.
type ClientConn struct {
	AssignedID      int
	Conn            net.Conn
	Send            chan []byte // serialized write channel
	Hub             *Hub
	LastSeenUpdated time.Time
}

// AdminConn wraps an admin browser WebSocket connection.
type AdminConn struct {
	Conn *websocket.Conn
	Send chan []byte
	Hub  *Hub
}

// Hub maintains the set of active client and admin connections.
type Hub struct {
	mu         sync.RWMutex
	clients    map[int]*ClientConn
	admins     map[*AdminConn]bool
	register   chan *ClientConn
	unregister chan *ClientConn
	adminReg   chan *AdminConn
	adminUnreg chan *AdminConn
	deviceRepo *data.DeviceRepo
}

// NewHub creates a new Hub and starts its run loop.
func NewHub(deviceRepo *data.DeviceRepo) *Hub {
	h := &Hub{
		clients:    make(map[int]*ClientConn),
		admins:     make(map[*AdminConn]bool),
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		adminReg:   make(chan *AdminConn),
		adminUnreg: make(chan *AdminConn),
		deviceRepo: deviceRepo,
	}
	go h.Run()
	return h
}

// Run is the main hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.AssignedID] = client
			h.mu.Unlock()
			if err := h.deviceRepo.UpdateConnected(client.AssignedID, true); err != nil {
				log.Printf("[hub] failed to mark device %d online: %v", client.AssignedID, err)
			}
			log.Printf("[hub] device %d connected", client.AssignedID)
			h.broadcastAdminEvent("device_connected", map[string]interface{}{
				"assigned_id": client.AssignedID,
			})

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.AssignedID]; ok {
				delete(h.clients, client.AssignedID)
				close(client.Send)
			}
			h.mu.Unlock()
			if err := h.deviceRepo.UpdateConnected(client.AssignedID, false); err != nil {
				log.Printf("[hub] failed to mark device %d offline: %v", client.AssignedID, err)
			}
			log.Printf("[hub] device %d disconnected", client.AssignedID)
			h.broadcastAdminEvent("device_disconnected", map[string]interface{}{
				"assigned_id": client.AssignedID,
			})

		case admin := <-h.adminReg:
			h.mu.Lock()
			h.admins[admin] = true
			adminCount := len(h.admins)
			h.mu.Unlock()
			log.Printf("[hub] admin connected (%d total)", adminCount)

		case admin := <-h.adminUnreg:
			h.mu.Lock()
			if _, ok := h.admins[admin]; ok {
				delete(h.admins, admin)
				close(admin.Send)
			}
			adminCount := len(h.admins)
			h.mu.Unlock()
			log.Printf("[hub] admin disconnected (%d remaining)", adminCount)
		}
	}
}

func (h *Hub) Register(client *ClientConn) {
	// Start write pump before registering.
	go func() {
		for msg := range client.Send {
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := client.Conn.Write(msg); err != nil {
				client.Conn.Close()
				// Drain the channel until it is closed by Unregister
				for range client.Send {
				}
				return
			}
		}
	}()
	h.register <- client
}
func (h *Hub) Unregister(client *ClientConn) { h.unregister <- client }

func (h *Hub) RegisterAdmin(admin *AdminConn)   { h.adminReg <- admin }
func (h *Hub) UnregisterAdmin(admin *AdminConn) { h.adminUnreg <- admin }

func (h *Hub) GetClient(assignedID int) *ClientConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[assignedID]
}

func (h *Hub) IsOnline(assignedID int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[assignedID]
	return ok
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetAllClients returns a copy of the connected clients slice.
func (h *Hub) GetAllClients() []*ClientConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*ClientConn, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	return clients
}

// BroadcastToClients sends a message to all connected TCP clients.
func (h *Hub) BroadcastToClients(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

// Kick closes a single client's TCP connection, forcing it to reconnect.
func (h *Hub) Kick(assignedID int) {
	h.mu.RLock()
	client, ok := h.clients[assignedID]
	h.mu.RUnlock()
	if ok {
		client.Conn.Close()
		log.Printf("[hub] kicked device %d", assignedID)
	}
}

// KickAll closes all client TCP connections, forcing them to reconnect.
func (h *Hub) KickAll() {
	h.mu.RLock()
	clients := make([]*ClientConn, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.Conn.Close()
	}
	log.Printf("[hub] kicked %d clients", len(clients))
}

func (h *Hub) BroadcastAdminEvent(event string, data interface{}) {
	h.broadcastAdminEvent(event, data)
}

func (h *Hub) broadcastAdminEvent(event string, data interface{}) {
	msg, err := json.Marshal(model.AdminEvent{Event: event, Data: data})
	if err != nil {
		log.Printf("[hub] failed to marshal admin event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for admin := range h.admins {
		select {
		case admin.Send <- msg:
		default:
		}
	}
}
