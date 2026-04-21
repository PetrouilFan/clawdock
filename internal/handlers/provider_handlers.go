package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"clawdock/internal/database"
	"clawdock/internal/models"
	"clawdock/internal/security"
)

// ProviderCreateRequest represents JSON payload for creating/updating a provider.
type ProviderCreateRequest struct {
	DisplayName            string `json:"display_name"`
	BaseURL                string `json:"base_url"`
	AuthType               string `json:"auth_type"`
	APIKey                 string `json:"api_key,omitempty"`
	Enabled                *bool  `json:"enabled,omitempty"`
	SupportsModelDiscovery *bool  `json:"supports_model_discovery,omitempty"`
}

// ProviderResponse is the public JSON representation (no encrypted key).
type ProviderResponse struct {
	ID                     string    `json:"id"`
	DisplayName            string    `json:"display_name"`
	BaseURL                *string   `json:"base_url,omitempty"`
	AuthType               string    `json:"auth_type"`
	Enabled                bool      `json:"enabled"`
	SupportsModelDiscovery bool      `json:"supports_model_discovery"`
	IsBuiltin              bool      `json:"is_builtin"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func toProviderResponse(p models.Provider) ProviderResponse {
	baseURL := p.BaseURL
	if baseURL == nil || *baseURL == "" {
		baseURL = nil
	}
	return ProviderResponse{
		ID:                     p.ID,
		DisplayName:            p.DisplayName,
		BaseURL:                baseURL,
		AuthType:               p.AuthType,
		Enabled:                p.Enabled,
		SupportsModelDiscovery: p.SupportsModelDiscovery,
		IsBuiltin:              p.IsBuiltin,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
	}
}

// CreateProvider handles POST /api/providers
func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req ProviderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.DisplayName) == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		http.Error(w, "base_url is required", http.StatusBadRequest)
		return
	}
	if !isValidURL(req.BaseURL) {
		http.Error(w, "base_url must start with http:// or https://", http.StatusBadRequest)
		return
	}
	allowedAuth := map[string]bool{"none": true, "api_key": true, "bearer": true}
	if !allowedAuth[req.AuthType] {
		http.Error(w, "auth_type must be one of: none, api_key, bearer", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	now := time.Now()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	supportsDiscovery := false
	if req.SupportsModelDiscovery != nil {
		supportsDiscovery = *req.SupportsModelDiscovery
	}

	// Encrypt API key if provided
	var apiKeyEncrypted string
	if req.APIKey != "" {
		secretKey := h.cfg.Security.SecretKey
		if secretKey == "" {
			http.Error(w, "server not configured with secret key", http.StatusInternalServerError)
			return
		}
		enc, err := security.Encrypt(req.APIKey, []byte(secretKey))
		if err != nil {
			http.Error(w, "failed to encrypt api key", http.StatusInternalServerError)
			return
		}
		apiKeyEncrypted = enc
	}

	_, err := h.db.Exec(`INSERT INTO providers (id, display_name, base_url, auth_type, enabled, supports_model_discovery, api_key_encrypted, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, req.DisplayName, req.BaseURL, req.AuthType, enabled, supportsDiscovery, apiKeyEncrypted, now, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "create_provider", id, fmt.Sprintf("Created provider %s", req.DisplayName), "success", nil)

	// Return the created provider (without API key)
	var p models.Provider
	err = h.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, id).Scan(
		&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		http.Error(w, "provider created but failed to fetch", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toProviderResponse(p))
}

// GetProvider handles GET /api/providers/:id
func (h *Handler) GetProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var p models.Provider
	err := h.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, api_key_encrypted, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, id).Scan(
		&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.APIKeyEncrypted, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := toProviderResponse(p)
	// Mask API key if present for display? Usually not returned; but admin UI edit might need to know it exists.
	// Not including api_key_encrypted in response at all.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// UpdateProvider handles PATCH /api/providers/:id
func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req ProviderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch existing provider to check built-in and current values
	var existing models.Provider
	err := h.db.QueryRow("SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, api_key_encrypted FROM providers WHERE id = ?", id).
		Scan(&existing.ID, &existing.DisplayName, &existing.BaseURL, &existing.AuthType, &existing.Enabled, &existing.SupportsModelDiscovery, &existing.IsBuiltin, &existing.APIKeyEncrypted)
	if err == sql.ErrNoRows {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Protect built-in: can edit but not change is_builtin; still allow updates to other fields.
	// We'll allow PATCH of any field except is_builtin and id.

	// Build UPDATE statement dynamically based on provided fields
	// We'll use a safe whitelist approach: fields present in req are updated.
	// Note: API key update: if provided, re-encrypt.

	query := "UPDATE providers SET updated_at = ?"
	args := []interface{}{time.Now()}

	if req.DisplayName != "" {
		query += ", display_name = ?"
		args = append(args, req.DisplayName)
	}
	if req.BaseURL != "" {
		if !isValidURL(req.BaseURL) {
			http.Error(w, "base_url must start with http:// or https://", http.StatusBadRequest)
			return
		}
		query += ", base_url = ?"
		args = append(args, req.BaseURL)
	}
	if req.AuthType != "" {
		allowedAuth := map[string]bool{"none": true, "api_key": true, "bearer": true}
		if !allowedAuth[req.AuthType] {
			http.Error(w, "auth_type must be one of: none, api_key, bearer", http.StatusBadRequest)
			return
		}
		query += ", auth_type = ?"
		args = append(args, req.AuthType)
	}
	if req.Enabled != nil {
		query += ", enabled = ?"
		args = append(args, *req.Enabled)
	}
	if req.SupportsModelDiscovery != nil {
		query += ", supports_model_discovery = ?"
		args = append(args, *req.SupportsModelDiscovery)
	}
	if req.APIKey != "" {
		secretKey := h.cfg.Security.SecretKey
		if secretKey == "" {
			http.Error(w, "server not configured with secret key", http.StatusInternalServerError)
			return
		}
		enc, err := security.Encrypt(req.APIKey, []byte(secretKey))
		if err != nil {
			http.Error(w, "failed to encrypt api key", http.StatusInternalServerError)
			return
		}
		query += ", api_key_encrypted = ?"
		args = append(args, enc)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	_, err = h.db.Exec(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "update_provider", id, "Updated provider", "success", req)

	// Return updated provider
	var p models.Provider
	err = h.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, id).Scan(
		&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		http.Error(w, "provider updated but failed to fetch", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toProviderResponse(p))
}

// DeleteProvider handles DELETE /api/providers/:id
func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Check if built-in
	var isBuiltin bool
	err := h.db.QueryRow("SELECT is_builtin FROM providers WHERE id = ?", id).Scan(&isBuiltin)
	if err == sql.ErrNoRows {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if isBuiltin {
		http.Error(w, "cannot delete built-in provider", http.StatusForbidden)
		return
	}

	_, err = h.db.Exec("DELETE FROM providers WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "delete_provider", id, "Deleted provider", "success", nil)
	w.WriteHeader(http.StatusOK)
}

// ListProviders handles GET /api/providers
func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var providers []models.Provider
	for rows.Next() {
		var p models.Provider
		var baseURL sql.NullString
		err := rows.Scan(&p.ID, &p.DisplayName, &baseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		if baseURL.Valid {
			p.BaseURL = &baseURL.String
		} else {
			p.BaseURL = nil
		}
		providers = append(providers, p)
	}

	var resp []ProviderResponse
	for _, p := range providers {
		resp = append(resp, toProviderResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListProviderModels handles GET /api/providers/{id}/models
func (h *Handler) ListProviderModels(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["id"]

	rows, err := h.db.Query(`SELECT id, provider_id, model_key, display_name, enabled, sort_order, last_health_check, health_status
		FROM provider_models WHERE provider_id = ? ORDER BY sort_order`, providerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var result []models.ProviderModel
	for rows.Next() {
		var m models.ProviderModel
		var lastHealthCheck sql.NullString
		err := rows.Scan(&m.ID, &m.ProviderID, &m.ModelKey, &m.DisplayName, &m.Enabled, &m.SortOrder, &lastHealthCheck, &m.HealthStatus)
		if err != nil {
			continue
		}
		if lastHealthCheck.Valid {
			t, _ := time.Parse(time.RFC3339, lastHealthCheck.String)
			m.LastHealthCheck = &t
		}
		result = append(result, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// UpdateProviderModel handles PATCH /api/provider-models/{id}
func (h *Handler) UpdateProviderModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["id"]

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if model exists
	var count int
	err := h.db.QueryRow("SELECT COUNT(1) FROM provider_models WHERE id = ?", modelID).Scan(&count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count == 0 {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	_, err = h.db.Exec("UPDATE provider_models SET enabled = ?, updated_at = ? WHERE id = ?", input.Enabled, time.Now(), modelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Include model_key in audit for clarity
	var modelKey string
	h.db.QueryRow("SELECT model_key FROM provider_models WHERE id = ?", modelID).Scan(&modelKey)
	database.AuditLogEntry(h.db, "system", "update_provider_model", modelID, fmt.Sprintf("Model %s enabled=%v", modelKey, input.Enabled), "success", nil)

	w.WriteHeader(http.StatusOK)
}

// RefreshProviderModels handles POST /api/providers/:id/refresh-models
func (h *Handler) RefreshProviderModels(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	result, err := h.providerRegistry.DiscoverAndUpsertModels(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetModelStatus handles GET /api/models/status
func (h *Handler) GetModelStatus(w http.ResponseWriter, r *http.Request) {
	// Refresh health for all providers first
	h.providerRegistry.CheckAllHealth()

	// Gather status for all provider models and custom models
	providers, _ := h.providerRegistry.ListAllProviders()
	type StatusEntry struct {
		ModelKey     string `json:"model_key"`
		ProviderID   string `json:"provider_id"`
		HealthStatus string `json:"health_status"`
	}
	var entries []StatusEntry
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		models, _ := h.providerRegistry.ListModelsForProvider(p.ID)
		for _, m := range models {
			entries = append(entries, StatusEntry{
				ModelKey:     m.ModelKey,
				ProviderID:   p.ID,
				HealthStatus: m.HealthStatus,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// isValidURL checks if URL starts with http:// or https://
func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}
