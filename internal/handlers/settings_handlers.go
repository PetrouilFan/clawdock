package handlers

import (
	"encoding/json"
	"net/http"

	"clawdock/internal/database"
)

// GetDefaultModel GET /api/settings/default_model
func (h *Handler) GetDefaultModel(w http.ResponseWriter, r *http.Request) {
	model, err := h.providerRegistry.GetDefaultModel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"default_model": model})
}

// SetDefaultModel PUT /api/settings/default_model
func (h *Handler) SetDefaultModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DefaultModel string `json:"default_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.DefaultModel == "" {
		// Allow empty to clear
		err := database.SetSetting(h.db, "default_model", "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	// Validate model exists and enabled
	_, _, err := h.providerRegistry.ResolveModel(req.DefaultModel)
	if err != nil {
		http.Error(w, "invalid model: "+err.Error(), http.StatusBadRequest)
		return
	}
	err = h.providerRegistry.SetDefaultModel(req.DefaultModel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"default_model": req.DefaultModel})
}

// GetChatProxyEnabled GET /api/settings/chat_proxy_enabled
func (h *Handler) GetChatProxyEnabled(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.providerRegistry.IsChatProxyEnabled()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
}

// SetChatProxyEnabled PUT /api/settings/chat_proxy_enabled
func (h *Handler) SetChatProxyEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err := h.providerRegistry.SetChatProxyEnabled(req.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"enabled": req.Enabled})
}
