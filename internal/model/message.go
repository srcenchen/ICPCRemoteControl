package model

import "encoding/json"

// ---- Client -> Server ----

type RegisterRequest struct {
	Type       string `json:"type"`
	AssignedID *int   `json:"assigned_id,omitempty"`
	MacAddress string `json:"mac_address"`
	Hostname   string `json:"hostname"`
}

type SystemInfoMessage struct {
	Type       string            `json:"type"`
	AssignedID int               `json:"assigned_id"`
	Info       []json.RawMessage `json:"info"`
}

// CommandOutputMessage is streaming output from a running command.
type CommandOutputMessage struct {
	Type      string `json:"type"` // "command_output"
	CommandID int64  `json:"command_id"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
	Line      string `json:"line"`
}

// CommandResultMessage is sent when a command finishes.
type CommandResultMessage struct {
	Type       string `json:"type"` // "command_result"
	CommandID  int64  `json:"command_id"`
	Status     string `json:"status"`
	ErrorOutput string `json:"error_output,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

// TerminalOutputMessage sends terminal output to server.
type TerminalOutputMessage struct {
	Type      string `json:"type"` // "terminal_output"
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

// TerminalClosedMessage indicates the terminal session ended.
type TerminalClosedMessage struct {
	Type      string `json:"type"` // "terminal_closed"
	SessionID string `json:"session_id"`
}

type PingMessage struct {
	Type string `json:"type"` // "ping"
}

// ---- Server -> Client ----

type RegisterResponse struct {
	Type       string `json:"type"`
	AssignedID int    `json:"assigned_id"`
}

type ExecuteMessage struct {
	Type      string `json:"type"` // "execute"
	CommandID int64  `json:"command_id"`
	Command   string `json:"command"`
}

type CancelMessage struct {
	Type      string `json:"type"` // "cancel"
	CommandID int64  `json:"command_id"`
}

// TerminalOpenMessage asks client to start an interactive shell.
type TerminalOpenMessage struct {
	Type      string `json:"type"` // "terminal_open"
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// TerminalInputMessage sends stdin to a terminal session.
type TerminalInputMessage struct {
	Type      string `json:"type"` // "terminal_input"
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

// TerminalResizeMessage changes terminal dimensions.
type TerminalResizeMessage struct {
	Type      string `json:"type"` // "terminal_resize"
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// TerminalCloseMessage asks client to close a terminal session.
type TerminalCloseMessage struct {
	Type      string `json:"type"` // "terminal_close"
	SessionID string `json:"session_id"`
}

type AckMessage struct {
	Type    string `json:"type"` // "ack"
	Message string `json:"message,omitempty"`
}

type PongMessage struct {
	Type string `json:"type"` // "pong"
}

// ---- Admin WebSocket (Server -> Browser) ----

type AdminEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// CommandOutputEvent is the streaming output sent to admin browsers.
type CommandOutputEvent struct {
	CommandID int64  `json:"command_id"`
	DeviceID  int    `json:"device_id"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
}
