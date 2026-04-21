package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"clawdock/internal/database"
)

// Custom model create/update request
type CustomModelRequest struct {
	TargetProviderID string `json:"target_provider_id"`
	TargetModelKey   string `json:"target_model_key"`
	Enabled          *bool  `json:"enabled,omitempty"`
}

// ListCustomModels GET /api/custom-models
func (h *Handler) ListCustomModels(w http.ResponseWriter, r *http.Request) {
	list, err := h.providerRegistry.ListAllCustomModels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// CreateCustomModel POST /api/custom-models/:alias  (alias in path)
func (h *Handler) CreateCustomModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alias := vars["alias"]

	var req CustomModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.TargetProviderID == "" || req.TargetModelKey == "" {
		http.Error(w, "target_provider_id and target_model_key required", http.StatusBadRequest)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	err := h.providerRegistry.CreateCustomModel(alias, req.TargetProviderID, req.TargetModelKey, enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c, _ := h.providerRegistry.GetCustomModel(alias)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

// GetCustomModel GET /api/custom-models/:alias
func (h *Handler) GetCustomModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alias := vars["alias"]
	c, err := h.providerRegistry.GetCustomModel(alias)
	if err != nil {
		if err.Error() == "custom model not found" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

// UpdateCustomModel PATCH /api/custom-models/:alias
func (h *Handler) UpdateCustomModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alias := vars["alias"]

	var req CustomModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Retrieve existing to audit
	existing, err := h.providerRegistry.GetCustomModel(alias)
	if err != nil {
		http.Error(w, "custom model not found", http.StatusNotFound)
		return
	}
	// Determine new values
	targetProviderID := existing.TargetProviderID
	targetModelKey := existing.TargetModelKey
	enabled := existing.Enabled
	if req.TargetProviderID != "" {
		targetProviderID = req.TargetProviderID
	}
	if req.TargetModelKey != "" {
		targetModelKey = req.TargetModelKey
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	err = h.providerRegistry.UpdateCustomModel(alias, targetProviderID, targetModelKey, enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := h.providerRegistry.GetCustomModel(alias)
	database.AuditLogEntry(h.db, "system", "update_custom_model", alias, fmt.Sprintf("Updated alias %s -> %s/%s enabled=%v", alias, targetProviderID, targetModelKey, enabled), "success", nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteCustomModel DELETE /api/custom-models/:alias
func (h *Handler) DeleteCustomModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alias := vars["alias"]
	err := h.providerRegistry.DeleteCustomModel(alias)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	database.AuditLogEntry(h.db, "system", "delete_custom_model", alias, "Deleted custom model alias", "success", nil)
	w.WriteHeader(http.StatusOK)
}
