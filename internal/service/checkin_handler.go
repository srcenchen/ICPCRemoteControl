package service

import (
	"encoding/json"
	"net/http"
	"strconv"

	"ICPCRemoteControl/internal/data"
)

// CheckinHandler handles check-in management REST API endpoints.
type CheckinHandler struct {
	repo *data.DeviceRepo
}

// NewCheckinHandler creates a new CheckinHandler.
func NewCheckinHandler(repo *data.DeviceRepo) *CheckinHandler {
	return &CheckinHandler{repo: repo}
}

// List returns all devices with check-in info (GET /api/checkin).
func (h *CheckinHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.repo.GetCheckinAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

// Stats returns check-in statistics (GET /api/checkin/stats).
func (h *CheckinHandler) Stats(w http.ResponseWriter, r *http.Request) {
	total, checkedIn, checkedOut, err := h.repo.GetCheckinStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":       total,
		"checked_in":  checkedIn,
		"checked_out": checkedOut,
		"not_checked": total - checkedIn - checkedOut,
	})
}

// DoCheckin performs check-in for a device (POST /api/checkin/{id}/checkin).
func (h *CheckinHandler) DoCheckin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	var body struct {
		StudentName string `json:"student_name"`
		StudentNum  string `json:"student_num"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.StudentName == "" || body.StudentNum == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "student_name and student_num are required"})
		return
	}

	if err := h.repo.Checkin(id, body.StudentName, body.StudentNum); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "checkin success"})
}

// DoCheckout performs check-out for a device (POST /api/checkin/{id}/checkout).
func (h *CheckinHandler) DoCheckout(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	if err := h.repo.Checkout(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "checkout success"})
}

// Reset resets a device's check-in status (POST /api/checkin/{id}/reset).
func (h *CheckinHandler) Reset(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}

	if err := h.repo.ResetCheckin(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "checkin reset"})
}

// Swap moves check-in info from one device to another (POST /api/checkin/swap).
func (h *CheckinHandler) Swap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FromAssignedID int `json:"from_assigned_id"`
		ToAssignedID   int `json:"to_assigned_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.FromAssignedID <= 0 || body.ToAssignedID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from_assigned_id and to_assigned_id are required"})
		return
	}

	if err := h.repo.SwapCheckin(body.FromAssignedID, body.ToAssignedID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "swap success"})
}
