package service

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"ICPCRemoteControl/internal/biz"
	"ICPCRemoteControl/internal/model"

)

// TerminalWSHandler handles browser WebSocket connections for interactive terminal.
type TerminalWSHandler struct {
	hub *biz.Hub
}

func NewTerminalWSHandler(hub *biz.Hub) *TerminalWSHandler {
	return &TerminalWSHandler{hub: hub}
}

// Serve handles browser terminal WebSocket (GET /ws/terminal/{device_id}).
func (h *TerminalWSHandler) Serve(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	deviceID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid device id", http.StatusBadRequest)
		return
	}

	client := h.hub.GetClient(deviceID)
	if client == nil {
		http.Error(w, "device not connected", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[terminal-ws] upgrade: %v", err)
		return
	}
	defer conn.Close()

	sessionID := "term_" + idStr

	// Subscribe to terminal output.
	TerminalHub.Subscribe(sessionID, conn)
	defer TerminalHub.Unsubscribe(sessionID, conn)

	// Open terminal on client.
	cols := 80
	rows := 24
	if c := r.URL.Query().Get("cols"); c != "" {
		if v, err := strconv.Atoi(c); err == nil && v > 0 {
			cols = v
		}
	}
	if rs := r.URL.Query().Get("rows"); rs != "" {
		if v, err := strconv.Atoi(rs); err == nil && v > 0 {
			rows = v
		}
	}

	openMsg := model.TerminalOpenMessage{
		Type:      "terminal_open",
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
	}
	data, _ := json.Marshal(openMsg)
	data = append(data, '\n')
	select { case client.Send <- data: default: }

	// Read from browser and forward to client as terminal_input / terminal_resize.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Check if it's a resize control message (JSON) or raw terminal input.
		if len(msg) > 0 && msg[0] == '{' {
			var ctrl struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
				resizeMsg := model.TerminalResizeMessage{
					Type:      "terminal_resize",
					SessionID: sessionID,
					Cols:      ctrl.Cols,
					Rows:      ctrl.Rows,
				}
				data, _ := json.Marshal(resizeMsg)
				data = append(data, '\n')
				select { case client.Send <- data: default: }
				continue
			}
		}

		// Raw terminal input.
		inputMsg := model.TerminalInputMessage{
			Type:      "terminal_input",
			SessionID: sessionID,
			Data:      string(msg),
		}
		data, _ := json.Marshal(inputMsg)
		data = append(data, '\n')
		select { case client.Send <- data: default: }
	}

	// Close terminal on client.
	closeMsg := model.TerminalCloseMessage{
		Type:      "terminal_close",
		SessionID: sessionID,
	}
	data, _ = json.Marshal(closeMsg)
	data = append(data, '\n')
	select { case client.Send <- data: default: }
}
