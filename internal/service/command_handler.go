package service

import (
	"encoding/json"
	"net/http"
	"strconv"

	"ICPCRemoteControl/internal/biz"
	"ICPCRemoteControl/internal/data"
	"ICPCRemoteControl/internal/model"
)

// CommandHandler handles REST API requests for command execution.
type CommandHandler struct {
	repo       *data.CommandRepo
	dispatcher *biz.CommandDispatcher
	hub        *biz.Hub
}

// NewCommandHandler creates a new CommandHandler.
func NewCommandHandler(repo *data.CommandRepo, dispatcher *biz.CommandDispatcher, hub *biz.Hub) *CommandHandler {
	return &CommandHandler{repo: repo, dispatcher: dispatcher, hub: hub}
}

// ExecuteRequest is the JSON body for POST /api/commands.
type ExecuteRequest struct {
	TargetType string `json:"target_type"`         // "single" or "broadcast"
	TargetID   *int   `json:"target_id,omitempty"` // required for single
	Command    string `json:"command"`
}

// Execute runs a command on the specified target(s) (POST /api/commands).
func (h *CommandHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Command == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "command is required"})
		return
	}
	if req.TargetType != "single" && req.TargetType != "broadcast" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_type must be 'single' or 'broadcast'"})
		return
	}
	if req.TargetType == "single" && req.TargetID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_id is required for single target"})
		return
	}

	cmd := &model.CommandLog{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Command:    req.Command,
		Status:     model.CommandStatusDispatched,
	}

	if err := h.repo.Create(cmd); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Dispatch asynchronously.
	go func() {
		if cmd.TargetType == "broadcast" {
			h.dispatcher.DispatchBroadcast(cmd)
		} else {
			h.dispatcher.DispatchSingle(cmd)
		}
	}()

	writeJSON(w, http.StatusCreated, cmd)
}

// List returns paginated command history (GET /api/commands?limit=50&offset=0).
func (h *CommandHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	cmds, err := h.repo.GetAll(limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cmds)
}

// Get returns a single command by ID (GET /api/commands/{id}).
// For broadcast commands, includes child results.
func (h *CommandHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid command id"})
		return
	}

	cmd, err := h.repo.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "command not found"})
		return
	}

	// For broadcast parents, populate children.
	if cmd.TargetType == "broadcast" {
		children, err := h.repo.GetByParentID(cmd.ID)
		if err == nil {
			cmd.Children = children
		}
	}

	writeJSON(w, http.StatusOK, cmd)
}

// Clear deletes all command history (POST /api/commands/clear).
func (h *CommandHandler) Clear(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.ClearAll(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "command history cleared"})
}

// Cancel sends a cancel signal to terminate a running command (POST /api/commands/{id}/cancel).
func (h *CommandHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid command id"})
		return
	}

	cmd, err := h.repo.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "command not found"})
		return
	}

	// If broadcast, cancel all children.
	if cmd.TargetType == "broadcast" {
		children, _ := h.repo.GetByParentID(cmd.ID)
		for _, child := range children {
			h.sendCancelToClient(child)
		}
	} else {
		h.sendCancelToClient(cmd)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "cancel sent"})
}

func (h *CommandHandler) sendCancelToClient(cmd *model.CommandLog) {
	if cmd.TargetID == nil {
		return
	}
	client := h.hub.GetClient(*cmd.TargetID)
	if client == nil {
		return
	}
	msg := model.CancelMessage{Type: "cancel", CommandID: cmd.ID}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	select {
	case client.Send <- data:
	default:
	}
}

// PresetCommand is a named preset command.
type PresetCommand struct {
	Name    string `json:"name"`
	Desc    string `json:"desc"`
	Command string `json:"command"`
	Color   string `json:"color"` // button color hint: danger, warning, success, primary
}

// Presets returns the list of preset commands (GET /api/presets).
func (h *CommandHandler) Presets(w http.ResponseWriter, r *http.Request) {
	presets := []PresetCommand{
		{
			Name:    "锁定键鼠",
			Desc:    "禁止所有输入设备（键盘、鼠标）",
			Command: "mkdir -p /etc/udev/rules.d && echo 'ACTION==\"add|change\", SUBSYSTEM==\"input\", ENV{LIBINPUT_IGNORE_DEVICE}=\"1\"' > /etc/udev/rules.d/99-icpc-lock.rules && udevadm control --reload-rules && udevadm trigger --subsystem-match=input && echo '键鼠已锁定'",
			Color:   "danger",
		},
		{
			Name:    "解锁键鼠",
			Desc:    "恢复所有输入设备",
			Command: "rm -f /etc/udev/rules.d/99-icpc-lock.rules && udevadm control --reload-rules && udevadm trigger --subsystem-match=input && echo '键鼠已解锁'",
			Color:   "success",
		},
		{
			Name:    "锁屏",
			Desc:    "锁定屏幕（需要桌面环境支持）",
			Command: "loginctl lock-sessions 2>/dev/null || xdg-screensaver lock 2>/dev/null || echo '无法锁屏'",
			Color:   "warning",
		},
		{
			Name:    "解锁",
			Desc:    "锁定屏幕（需要桌面环境支持）",
			Command: "loginctl unlock-sessions 2>/dev/null || \\\nDISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(pgrep -u $(whoami) -x kded5 || echo 1000)/bus qdbus org.freedesktop.ScreenSaver /ScreenSaver SetActive false 2>/dev/null || \\\npkill -9 -f kscreenlocker_greet 2>/dev/null || \\\necho '无法解锁'",
			Color:   "success",
		},
		{
			Name:    "关机",
			Desc:    "立即关闭选手机",
			Command: "shutdown now",
			Color:   "danger",
		},
		{
			Name:    "重启",
			Desc:    "立即重启选手机",
			Command: "reboot",
			Color:   "warning",
		},
		{
			Name:    "同步时间",
			Desc:    "从服务器同步系统时间",
			Command: "timedatectl set-ntp true 2>/dev/null && echo 'NTP 已启用' || echo '时间同步设置失败'",
			Color:   "primary",
		},
	}
	writeJSON(w, http.StatusOK, presets)
}
