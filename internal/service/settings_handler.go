package service

import (
	"encoding/json"
	"net/http"
	"strings"
)

// SettingsHandler handles the settings API endpoints.
type SettingsHandler struct {
	settings *ServerSettings
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(settings *ServerSettings) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

// Get returns current settings (GET /api/settings).
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.settings.Snapshot())
}

// Update accepts partial settings updates (POST /api/settings).
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostnamePrefix *string `json:"hostname_prefix,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.HostnamePrefix != nil {
		prefix := strings.TrimSpace(*req.HostnamePrefix)
		if prefix == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname_prefix cannot be empty"})
			return
		}
		if len(prefix) > 64 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname_prefix too long (max 64)"})
			return
		}
		h.settings.SetHostnamePrefix(prefix)
	}

	writeJSON(w, http.StatusOK, h.settings.Snapshot())
}

// GetPresets returns current preset commands (GET /api/settings/presets).
func (h *SettingsHandler) GetPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.settings.GetPresets())
}

// GetCheckinConfig returns the check-in page configuration (GET /api/settings/checkin).
func (h *SettingsHandler) GetCheckinConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.settings.GetCheckinConfig())
}

// UpdateCheckinConfig updates the check-in page configuration (PUT /api/settings/checkin).
func (h *SettingsHandler) UpdateCheckinConfig(w http.ResponseWriter, r *http.Request) {
	var cfg CheckinConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(cfg.PostCheckoutCmd) > 4096 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "post_checkout_cmd too long"})
		return
	}
	h.settings.SetCheckinConfig(cfg)
	writeJSON(w, http.StatusOK, h.settings.GetCheckinConfig())
}

// UpdatePresets replaces the entire presets list (PUT /api/settings/presets).
func (h *SettingsHandler) UpdatePresets(w http.ResponseWriter, r *http.Request) {
	var presets []PresetCommand
	if err := json.NewDecoder(r.Body).Decode(&presets); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if presets == nil {
		presets = make([]PresetCommand, 0)
	}

	// Validate each preset.
	for i, p := range presets {
		if strings.TrimSpace(p.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "preset name cannot be empty"})
			return
		}
		if strings.TrimSpace(p.Command) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "preset command cannot be empty"})
			return
		}
		if len(p.Command) > 64*1024 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "preset command too long"})
			return
		}
		if p.Color == "" {
			presets[i].Color = "primary"
		}
		presets[i].Name = strings.TrimSpace(p.Name)
		presets[i].Desc = strings.TrimSpace(p.Desc)
		presets[i].Command = strings.TrimSpace(p.Command)
	}

	h.settings.SetPresets(presets)
	writeJSON(w, http.StatusOK, h.settings.GetPresets())
}
