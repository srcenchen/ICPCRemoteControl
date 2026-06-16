package service

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var broadcastWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// BroadcastWSHub manages WebSocket connections from broadcast display pages.
type BroadcastWSHub struct {
	mu         sync.RWMutex
	conns      map[string]map[*websocket.Conn]bool
	carousel   map[string]chan struct{}
	carouselMu sync.Mutex
	// Carousel page provider (set by handler).
	PageProvider func(mode string) []pageInfo
}

type pageInfo struct {
	DurationMs int
}

var BroadcastWS = &BroadcastWSHub{
	conns:    make(map[string]map[*websocket.Conn]bool),
	carousel: make(map[string]chan struct{}),
}

func (h *BroadcastWSHub) StartCarousel(mode string) {
	h.carouselMu.Lock()
	if stop, ok := h.carousel[mode]; ok {
		close(stop)
	}
	stop := make(chan struct{})
	h.carousel[mode] = stop
	h.carouselMu.Unlock()

	go h.runCarousel(mode, stop)
}

func (h *BroadcastWSHub) StopCarousel(mode string) {
	h.carouselMu.Lock()
	if stop, ok := h.carousel[mode]; ok {
		close(stop)
		delete(h.carousel, mode)
	}
	h.carouselMu.Unlock()
}

func (h *BroadcastWSHub) runCarousel(mode string, stop chan struct{}) {
	var pageIdx int

	// Initial delay to let pages load.
	select {
	case <-stop:
		return
	case <-time.After(500 * time.Millisecond):
	}

	for {
		if h.PageProvider == nil {
			select {
			case <-stop: return
			case <-time.After(time.Second):
			}
			continue
		}
		pages := h.PageProvider(mode)
		if len(pages) == 0 {
			select {
			case <-stop: return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if pageIdx >= len(pages) {
			pageIdx = 0
		}

		// Send current page.
		msg, _ := json.Marshal(map[string]interface{}{
			"type":       "page_switch",
			"mode":       mode,
			"page_index": pageIdx,
			"total":      len(pages),
		})
		h.Broadcast(mode, msg)

		// Wait for page duration.
		dur := pages[pageIdx].DurationMs
		if dur <= 0 {
			dur = 10000
		}
		select {
		case <-stop:
			return
		case <-time.After(time.Duration(dur) * time.Millisecond):
		}

		pageIdx = (pageIdx + 1) % len(pages)
	}
}

func (h *BroadcastWSHub) Serve(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "before"
	}
	conn, err := broadcastWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[broadcast-ws] upgrade: %v", err)
		return
	}

	h.mu.Lock()
	if h.conns[mode] == nil {
		h.conns[mode] = make(map[*websocket.Conn]bool)
	}
	h.conns[mode][conn] = true
	h.mu.Unlock()

	log.Printf("[broadcast-ws] mode=%s connected (%d total)", mode, len(h.conns[mode]))

	// Send current pages immediately.
	// The caller (BroadcastHandler) needs access but we use a callback/interface instead.
	// For now just keep alive.

	defer func() {
		h.mu.Lock()
		if h.conns[mode] != nil {
			delete(h.conns[mode], conn)
			if len(h.conns[mode]) == 0 {
				delete(h.conns, mode)
			}
		}
		h.mu.Unlock()
		conn.Close()
		log.Printf("[broadcast-ws] mode=%s disconnected", mode)
	}()

	// Keep connection alive, read pings.
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// Broadcast sends a message to all display connections for a given mode.
func (h *BroadcastWSHub) Broadcast(mode string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.conns[mode] {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("[broadcast-ws] write error: %v", err)
		}
	}
}

// BroadcastAll sends to all modes.
func (h *BroadcastWSHub) BroadcastAll(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, conns := range h.conns {
		for conn := range conns {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[broadcast-ws] write error: %v", err)
			}
		}
	}
}
