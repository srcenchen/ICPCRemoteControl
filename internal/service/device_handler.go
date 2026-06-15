package service

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"ICPCRemoteControl/internal/biz"
	"ICPCRemoteControl/internal/data"
)

// DeviceHandler handles REST API requests for devices.
type DeviceHandler struct {
	repo *data.DeviceRepo
	hub  *biz.Hub
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(repo *data.DeviceRepo, hub *biz.Hub) *DeviceHandler {
	return &DeviceHandler{repo: repo, hub: hub}
}

// List returns all devices as JSON (GET /api/devices).
func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.repo.GetAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

// Get returns a single device by assigned ID (GET /api/devices/{id}).
func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	device, err := h.repo.GetByAssignedID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	writeJSON(w, http.StatusOK, device)
}

// Delete removes a device record (DELETE /api/devices/{id}).
func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	if err := h.repo.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// Reset clears all device records and kicks all connected clients (POST /api/devices/reset).
func (h *DeviceHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.ResetAll(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	log.Println("[device] all device records cleared")

	// Kick all connected clients so they reconnect with fresh IDs.
	if h.hub != nil {
		h.hub.KickAll()
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "all devices reset"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
