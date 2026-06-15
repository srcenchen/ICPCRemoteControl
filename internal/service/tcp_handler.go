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

// TCPHandler handles TCP connections from contestant machines.
type TCPHandler struct {
	hub         *biz.Hub
	deviceRepo  *data.DeviceRepo
	commandRepo *data.CommandRepo
	idAssigner  *biz.IDAssigner
	dispatcher  *biz.CommandDispatcher
	outputBuf   map[int64]string // command_id → accumulated streaming output
	obMu        sync.Mutex
}

func NewTCPHandler(hub *biz.Hub, deviceRepo *data.DeviceRepo, commandRepo *data.CommandRepo, idAssigner *biz.IDAssigner, dispatcher *biz.CommandDispatcher) *TCPHandler {
	return &TCPHandler{
		hub: hub, deviceRepo: deviceRepo, commandRepo: commandRepo,
		idAssigner: idAssigner, dispatcher: dispatcher,
		outputBuf: make(map[int64]string),
	}
}

func (h *TCPHandler) Handle(conn net.Conn) {
	defer conn.Close()

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
	regResp := model.RegisterResponse{Type: "register_response", AssignedID: assignedID}
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

	// --- Main loop ---
	conn.SetDeadline(time.Time{})

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("[tcp] device %d: read: %v", assignedID, err)
			}
			return
		}

		var base struct{ Type string }
		json.Unmarshal([]byte(line), &base)

		switch base.Type {
		case "command_output":
			var msg model.CommandOutputMessage
			json.Unmarshal([]byte(line), &msg)
			// Persist output to DB immediately so history works for running commands.
			h.obMu.Lock()
			h.outputBuf[msg.CommandID] += msg.Line + "\n"
			buf := h.outputBuf[msg.CommandID]
			h.obMu.Unlock()
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
			json.Unmarshal([]byte(line), &cr)
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
				"command_id":  cr.CommandID,
				"device_id":   assignedID,
				"status":      cr.Status,
				"error_output": cr.ErrorOutput,
				"duration_ms": cr.DurationMS,
			})

		case "terminal_output":
			var msg model.TerminalOutputMessage
			json.Unmarshal([]byte(line), &msg)
			TerminalHub.Broadcast(msg.SessionID, msg.Data)

		case "terminal_closed":
			var msg model.TerminalClosedMessage
			json.Unmarshal([]byte(line), &msg)
			TerminalHub.Broadcast(msg.SessionID, []byte("\x1b[31mSession closed\x1b[0m\r\n"))
			TerminalHub.Close(msg.SessionID)

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
			log.Printf("[tcp] new connection from %s", conn.RemoteAddr())
			go handler.Handle(conn)
		}
	}()
	return nil
}
