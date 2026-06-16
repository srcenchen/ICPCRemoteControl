package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"ICPCRemoteControl/internal/biz"
	"ICPCRemoteControl/internal/data"
	"ICPCRemoteControl/internal/model"
)

const maxTCPConns = 500

// TCPHandler handles TCP connections from contestant machines.
type TCPHandler struct {
	hub         *biz.Hub
	deviceRepo  *data.DeviceRepo
	commandRepo *data.CommandRepo
	idAssigner  *biz.IDAssigner
	dispatcher  *biz.CommandDispatcher
	outputBuf   map[int64]string // command_id → accumulated streaming output
	obMu        sync.Mutex
	connCount   int
	connMu      sync.Mutex
	settings    *ServerSettings
}

func NewTCPHandler(hub *biz.Hub, deviceRepo *data.DeviceRepo, commandRepo *data.CommandRepo, idAssigner *biz.IDAssigner, dispatcher *biz.CommandDispatcher, settings *ServerSettings) *TCPHandler {
	return &TCPHandler{
		hub: hub, deviceRepo: deviceRepo, commandRepo: commandRepo,
		idAssigner: idAssigner, dispatcher: dispatcher,
		outputBuf: make(map[int64]string),
		settings:  settings,
	}
}

func (h *TCPHandler) incConn() bool {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.connCount >= maxTCPConns {
		return false
	}
	h.connCount++
	return true
}

func (h *TCPHandler) decConn() {
	h.connMu.Lock()
	h.connCount--
	h.connMu.Unlock()
}

func (h *TCPHandler) Handle(conn net.Conn) {
	defer func() {
		conn.Close()
		h.decConn()
	}()

	conn.SetDeadline(time.Now().Add(30 * time.Second))
	reader := bufio.NewReader(conn)

	// --- Registration ---
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	var regReq model.RegisterRequest
	if err := json.Unmarshal([]byte(line), &regReq); err != nil || regReq.Type != "register_request" {
		return
	}

	// Atomic ID assignment — prevents concurrent clients getting the same ID.
	assignedID, existingDevice, err := h.idAssigner.AssignOrReuse(regReq.MacAddress, regReq.AssignedID)
	if err != nil {
		log.Printf("[tcp] id assignment error: %v", err)
		return
	}

	clientConn := &biz.ClientConn{
		AssignedID: assignedID,
		Conn:       conn,
		Send:       make(chan []byte, 64),
		Hub:        h.hub,
	}

	// Write register_response via Send channel (buffered, before write pump starts).
	regResp := model.RegisterResponse{Type: "register_response", AssignedID: assignedID, HostnamePrefix: h.settings.GetHostnamePrefix()}
	respData, _ := json.Marshal(regResp)
	respData = append(respData, '\n')
	clientConn.Send <- respData

	h.hub.Register(clientConn)
	defer h.hub.Unregister(clientConn)

	// --- System info ---
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	line, err = reader.ReadString('\n')
	if err != nil {
		return
	}
	var sysMsg model.SystemInfoMessage
	if err := json.Unmarshal([]byte(line), &sysMsg); err != nil || sysMsg.Type != "system_info" {
		return
	}

	rawJSON, _ := json.Marshal(sysMsg.Info)
	device, err := model.ParseFastFetch(rawJSON)
	if err != nil {
		device = &model.Device{AssignedID: assignedID, FastfetchRaw: string(rawJSON)}
	}
	device.AssignedID = assignedID
	device.MacAddress = regReq.MacAddress
	device.Connected = true

	if existingDevice != nil {
		device.ID = existingDevice.ID
		h.deviceRepo.Update(device)
	} else if existing, err := h.deviceRepo.GetByAssignedID(assignedID); err == nil {
		device.ID = existing.ID
		h.deviceRepo.Update(device)
	} else {
		h.deviceRepo.Create(device)
	}

	h.hub.BroadcastAdminEvent("device_updated", map[string]interface{}{"assigned_id": assignedID})
	log.Printf("[tcp] device %d registered", assignedID)

	// Push check-in config to client.
	cfg := h.settings.GetCheckinConfig()
	cfgData, _ := json.Marshal(model.CheckinConfigMessage{
		Type:            "checkin_config",
		WelcomeText:     cfg.WelcomeText,
		WarningText:     cfg.WarningText,
		PostCheckinMsg:  cfg.PostCheckinMsg,
		PostCheckoutCmd: cfg.PostCheckoutCmd,
		PostCheckoutMsg: cfg.PostCheckoutMsg,
	})
	cfgData = append(cfgData, '\n')
	select {
	case clientConn.Send <- cfgData:
	default:
	}

	// --- Main loop ---
	conn.SetDeadline(time.Time{})

	// Track in-flight command IDs for this device so we can fail them on disconnect.
	inFlightCmds := make(map[int64]bool)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[tcp] device %d: read: %v", assignedID, err)
			}
			h.failInFlightCommands(inFlightCmds)
			h.obMu.Lock()
			for cmdID := range inFlightCmds {
				delete(h.outputBuf, cmdID)
			}
			h.obMu.Unlock()
			return
		}

		var base struct{ Type string }
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			log.Printf("[tcp] device %d: unmarshal message type: %v", assignedID, err)
			continue
		}

		switch base.Type {
		case "command_output":
			var msg model.CommandOutputMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[tcp] device %d: unmarshal command_output: %v", assignedID, err)
				continue
			}
			// Persist output to DB immediately so history works for running commands.
			h.obMu.Lock()
			h.outputBuf[msg.CommandID] += msg.Line + "\n"
			buf := h.outputBuf[msg.CommandID]
			h.obMu.Unlock()
			inFlightCmds[msg.CommandID] = true
			// Update DB with accumulated output (lightweight, just a string update).
			if cmd, err := h.commandRepo.GetByID(msg.CommandID); err == nil {
				cmd.Output = buf
				h.commandRepo.UpdateStatus(cmd)
			}
			h.hub.BroadcastAdminEvent("command_output", model.CommandOutputEvent{
				CommandID: msg.CommandID,
				DeviceID:  assignedID,
				Stream:    msg.Stream,
				Line:      msg.Line,
			})

		case "command_result":
			var cr model.CommandResultMessage
			if err := json.Unmarshal([]byte(line), &cr); err != nil {
				log.Printf("[tcp] device %d: unmarshal command_result: %v", assignedID, err)
				continue
			}
			delete(inFlightCmds, cr.CommandID)
			cmd, err := h.commandRepo.GetByID(cr.CommandID)
			if err == nil {
				cmd.Status = cr.Status
				cmd.ErrorOutput = cr.ErrorOutput
				cmd.DurationMS = cr.DurationMS
				// Final accumulated output is already in DB from command_output handlers.
				h.obMu.Lock()
				if buf, ok := h.outputBuf[cr.CommandID]; ok {
					cmd.Output = buf
					delete(h.outputBuf, cr.CommandID)
				}
				h.obMu.Unlock()
				h.commandRepo.UpdateStatus(cmd)
				if cmd.ParentID != nil {
					h.dispatcher.UpdateBroadcastParentStatus(*cmd.ParentID)
				}
			}
			h.hub.BroadcastAdminEvent("command_result", map[string]interface{}{
				"command_id":   cr.CommandID,
				"device_id":    assignedID,
				"status":       cr.Status,
				"error_output": cr.ErrorOutput,
				"duration_ms":  cr.DurationMS,
			})

		case "terminal_output":
			var msg model.TerminalOutputMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[tcp] device %d: unmarshal terminal_output: %v", assignedID, err)
				continue
			}
			TerminalHub.Broadcast(msg.SessionID, msg.Data)

		case "terminal_closed":
			var msg model.TerminalClosedMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[tcp] device %d: unmarshal terminal_closed: %v", assignedID, err)
				continue
			}
			TerminalHub.Broadcast(msg.SessionID, []byte("\x1b[31mSession closed\x1b[0m\r\n"))
			TerminalHub.Close(msg.SessionID)

		case "checkin":
			var msg model.CheckinMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[tcp] device %d: unmarshal checkin: %v", assignedID, err)
				continue
			}
			err := h.deviceRepo.Checkin(assignedID, msg.StudentName, msg.StudentNum)
			cfg := h.settings.GetCheckinConfig()
			resp := model.CheckinResponseMessage{
				Type: "checkin_response", CorrelationID: msg.CorrelationID, Success: err == nil,
				PostCheckinMsg: cfg.PostCheckinMsg,
			}
			if err != nil {
				resp.Message = err.Error()
			} else {
				resp.Message = "checkin success"
			}
			respData, _ := json.Marshal(resp)
			respData = append(respData, '\n')
			select {
			case clientConn.Send <- respData:
			default:
			}
			log.Printf("[tcp] device %d: checkin name=%s num=%s success=%v", assignedID, msg.StudentName, msg.StudentNum, err == nil)

		case "checkin_query":
			var msg model.CheckinMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			resp := model.CheckinResponseMessage{
				Type: "checkin_response", CorrelationID: msg.CorrelationID, Success: true,
			}
			respData, _ := json.Marshal(resp)
			respData = append(respData, '\n')
			select {
			case clientConn.Send <- respData:
			default:
			}

		case "checkout":
			var msg model.CheckinMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			err := h.deviceRepo.Checkout(assignedID)
			cfg := h.settings.GetCheckinConfig()
			resp := model.CheckinResponseMessage{
				Type: "checkin_response", CorrelationID: msg.CorrelationID, Success: err == nil,
				PostCheckoutCmd: cfg.PostCheckoutCmd,
				PostCheckoutMsg: cfg.PostCheckoutMsg,
			}
			if err != nil {
				resp.Message = err.Error()
			}
			respData, _ := json.Marshal(resp)
			respData = append(respData, '\n')
			select {
			case clientConn.Send <- respData:
			default:
			}
			log.Printf("[tcp] device %d: checkout success=%v", assignedID, err == nil)

		case "ping":
			pongData, _ := json.Marshal(model.PongMessage{Type: "pong"})
			pongData = append(pongData, '\n')
			select {
			case clientConn.Send <- pongData:
			default:
			}

		default:
			log.Printf("[tcp] device %d: unknown type: %s", assignedID, base.Type)
		}
	}
}

// failInFlightCommands marks all in-flight commands as failed due to client disconnect.
func (h *TCPHandler) failInFlightCommands(inFlight map[int64]bool) {
	for cmdID := range inFlight {
		cmd, err := h.commandRepo.GetByID(cmdID)
		if err != nil {
			continue
		}
		// Only mark if still in a non-terminal state.
		if cmd.Status == model.CommandStatusDispatched || cmd.Status == model.CommandStatusPending {
			cmd.Status = model.CommandStatusFailed
			cmd.ErrorOutput = "client disconnected"
			h.commandRepo.UpdateStatus(cmd)
			if cmd.ParentID != nil {
				h.dispatcher.UpdateBroadcastParentStatus(*cmd.ParentID)
			}
		}
	}
}

// --- TCP listener ---

func StartTCPListener(addr string, handler *TCPHandler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("tcp listen: %w", err)
	}
	log.Printf("[tcp] listening on %s", addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("[tcp] accept: %v", err)
				continue
			}
			if !handler.incConn() {
				log.Printf("[tcp] connection limit reached (%d), rejecting %s", maxTCPConns, conn.RemoteAddr())
				conn.Close()
				continue
			}
			log.Printf("[tcp] new connection from %s (%d/%d)", conn.RemoteAddr(), handler.connCount, maxTCPConns)
			go handler.Handle(conn)
		}
	}()
	return nil
}
