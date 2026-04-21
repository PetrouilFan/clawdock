package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"clawdock/internal/database"
	"clawdock/internal/models"
)

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query("SELECT id, name, slug, enabled, image_repo, image_tag, provider_id, model_id, workspace_host_path, workspace_container_path, restart_policy, status_desired, status_actual, drift_state, last_error, last_reconciled_at, created_at, updated_at FROM agents WHERE deleted_at IS NULL")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		var lastReconciledAt sql.NullTime
		var lastError sql.NullString
		err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Enabled, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy, &a.StatusDesired, &a.StatusActual, &a.DriftState, &lastError, &lastReconciledAt, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			continue
		}
		if lastError.Valid {
			a.LastError = &lastError.String
		}
		if lastReconciledAt.Valid {
			a.LastReconciledAt = &lastReconciledAt.Time
		}
		agents = append(agents, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name                   string `json:"name"`
		ImageTag               string `json:"image_tag"`
		ProviderID             string `json:"provider_id"`
		ModelID                string `json:"model_id"`
		TelegramAPIKey         string `json:"telegram_api_key"`
		WorkspaceHostPath      string `json:"workspace_host_path"`
		WorkspaceContainerPath string `json:"workspace_container_path"`
		RestartPolicy          string `json:"restart_policy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate provider exists and is enabled
	var providerEnabled int
	err := h.db.QueryRow("SELECT enabled FROM providers WHERE id = ?", input.ProviderID).Scan(&providerEnabled)
	if err == sql.ErrNoRows {
		http.Error(w, "provider not found", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if providerEnabled == 0 {
		http.Error(w, "provider is disabled", http.StatusBadRequest)
		return
	}

	// Validate model exists and is enabled for that provider (direct or via custom alias)
	var modelCount int
	err = h.db.QueryRow("SELECT COUNT(1) FROM provider_models WHERE provider_id = ? AND model_key = ? AND enabled = 1", input.ProviderID, input.ModelID).Scan(&modelCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if modelCount == 0 {
		// Check if it's a custom model alias
		var customTargetProviderID, customTargetModelKey string
		customErr := h.db.QueryRow("SELECT target_provider_id, target_model_key FROM custom_models WHERE id = ? AND enabled = 1", input.ModelID).Scan(&customTargetProviderID, &customTargetModelKey)
		if customErr != nil {
			http.Error(w, "model not found or disabled", http.StatusBadRequest)
			return
		}
		if customTargetProviderID != input.ProviderID {
			http.Error(w, "custom model does not map to specified provider", http.StatusBadRequest)
			return
		}
		var targetEnabled int
		err = h.db.QueryRow("SELECT COUNT(1) FROM provider_models WHERE provider_id = ? AND model_key = ? AND enabled = 1", customTargetProviderID, customTargetModelKey).Scan(&targetEnabled)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if targetEnabled == 0 {
			http.Error(w, "custom model's target model is disabled", http.StatusBadRequest)
			return
		}
	}

	id := uuid.New().String()
	now := time.Now()
	slug := sanitizeSlug(input.Name)

	_, err = h.db.Exec(`INSERT INTO agents (id, name, slug, image_repo, image_tag, provider_id, model_id, telegram_api_key_encrypted, workspace_host_path, workspace_container_path, restart_policy, status_desired, status_actual, drift_state, created_at, updated_at)
		VALUES (?, ?, ?, 'ghcr.io/openclaw/openclaw', ?, ?, ?, ?, ?, ?, ?, 'stopped', 'unknown', 'unknown', ?, ?)`,
		id, input.Name, slug, input.ImageTag, input.ProviderID, input.ModelID, input.TelegramAPIKey,
		input.WorkspaceHostPath, input.WorkspaceContainerPath, input.RestartPolicy, now, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "create_agent", id, fmt.Sprintf("Created agent %s", input.Name), "success", nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}


func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var a models.Agent
	err := h.db.QueryRow("SELECT id, name, slug, enabled, image_repo, image_tag, provider_id, model_id, workspace_host_path, workspace_container_path, restart_policy, status_desired, status_actual, drift_state, created_at, updated_at FROM agents WHERE id = ? AND deleted_at IS NULL",
		id).Scan(&a.ID, &a.Name, &a.Slug, &a.Enabled, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy, &a.StatusDesired, &a.StatusActual, &a.DriftState, &a.CreatedAt, &a.UpdatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

func (h *Handler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := "UPDATE agents SET updated_at = ?, config_revision = config_revision + 1"
	args := []interface{}{time.Now()}

	if name, ok := input["name"]; ok {
		query += ", name = ?"
		args = append(args, name)
		query += ", slug = ?"
		args = append(args, sanitizeSlug(name.(string)))
	}
	if imageTag, ok := input["image_tag"]; ok {
		query += ", image_tag = ?"
		args = append(args, imageTag)
	}
	if providerID, ok := input["provider_id"]; ok {
		query += ", provider_id = ?"
		args = append(args, providerID)
	}
	if modelID, ok := input["model_id"]; ok {
		query += ", model_id = ?"
		args = append(args, modelID)
	}
	if workspaceHostPath, ok := input["workspace_host_path"]; ok {
		query += ", workspace_host_path = ?"
		args = append(args, workspaceHostPath)
	}
	if restartPolicy, ok := input["restart_policy"]; ok {
		query += ", restart_policy = ?"
		args = append(args, restartPolicy)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	_, err := h.db.Exec(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "update_agent", id, "Updated agent configuration", "success", input)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	mode := r.URL.Query().Get("mode")

	switch mode {
	case "container":
		h.docker.RemoveContainer(id)
		database.AuditLogEntry(h.db, "system", "delete_container", id, "Deleted agent container", "success", nil)
	case "metadata":
		h.db.Exec("UPDATE agents SET deleted_at = ? WHERE id = ?", time.Now(), id)
		database.AuditLogEntry(h.db, "system", "delete_agent", id, "Deleted agent metadata", "success", nil)
	case "full":
		h.docker.RemoveContainer(id)
		h.db.Exec("UPDATE agents SET deleted_at = ? WHERE id = ?", time.Now(), id)
		database.AuditLogEntry(h.db, "system", "delete_agent_full", id, "Deleted agent and all data", "success", nil)
	default:
		http.Error(w, "invalid mode", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) StartAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.docker.StartContainer(id); err != nil {
		database.AuditLogEntry(h.db, "system", "start_agent", id, "Failed to start agent", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.db.Exec("UPDATE agents SET status_desired = 'running', updated_at = ? WHERE id = ?", time.Now(), id)
	database.AuditLogEntry(h.db, "system", "start_agent", id, "Started agent", "success", nil)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) StopAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.docker.StopContainer(id); err != nil {
		database.AuditLogEntry(h.db, "system", "stop_agent", id, "Failed to stop agent", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.db.Exec("UPDATE agents SET status_desired = 'stopped', updated_at = ? WHERE id = ?", time.Now(), id)
	database.AuditLogEntry(h.db, "system", "stop_agent", id, "Stopped agent", "success", nil)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RestartAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	h.docker.StopContainer(id)
	h.docker.StartContainer(id)

	database.AuditLogEntry(h.db, "system", "restart_agent", id, "Restarted agent", "success", nil)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RecreateAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var a models.Agent
	err := h.db.QueryRow("SELECT id, name, slug, image_repo, image_tag, provider_id, model_id, telegram_api_key_encrypted, workspace_host_path, workspace_container_path, restart_policy FROM agents WHERE id = ? AND deleted_at IS NULL",
		id).Scan(&a.ID, &a.Name, &a.Slug, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.TelegramAPIKeyEncrypted, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy)

	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	h.docker.StopContainer(id)
	h.docker.RemoveContainer(id)

	containerID, err := h.docker.CreateAgentContainer(&a)
	if err != nil {
		database.AuditLogEntry(h.db, "system", "recreate_agent", id, "Failed to recreate agent", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.docker.StartContainer(containerID)

	h.db.Exec("UPDATE agents SET config_revision = config_revision + 1, updated_at = ? WHERE id = ?", time.Now(), id)
	database.AuditLogEntry(h.db, "system", "recreate_agent", id, "Recreated agent container", "success", nil)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CloneAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var input struct {
		Name              string `json:"name"`
		WorkspaceHostPath string `json:"workspace_host_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var a models.Agent
	err := h.db.QueryRow("SELECT id, name, slug, image_repo, image_tag, provider_id, model_id, telegram_api_key_encrypted, workspace_host_path, workspace_container_path, restart_policy FROM agents WHERE id = ? AND deleted_at IS NULL",
		id).Scan(&a.ID, &a.Name, &a.Slug, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.TelegramAPIKeyEncrypted, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy)

	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	newID := uuid.New().String()
	now := time.Now()
	newSlug := sanitizeSlug(input.Name)

	_, err = h.db.Exec(`INSERT INTO agents (id, name, slug, image_repo, image_tag, provider_id, model_id, telegram_api_key_encrypted, workspace_host_path, workspace_container_path, restart_policy, status_desired, status_actual, drift_state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'stopped', 'unknown', 'unknown', ?, ?)`,
		newID, input.Name, newSlug, a.ImageRepo, a.ImageTag, a.ProviderID, a.ModelID, a.TelegramAPIKeyEncrypted,
		input.WorkspaceHostPath, a.WorkspaceContainerPath, a.RestartPolicy, now, now)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "clone_agent", newID, fmt.Sprintf("Cloned agent from %s", a.Name), "success", map[string]string{"source_id": id})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": newID})
}

func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	logs, err := h.docker.GetContainerLogs(id, "500")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(logs))
}

func (h *Handler) RepairAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var a models.Agent
	err := h.db.QueryRow("SELECT id, name, slug, image_repo, image_tag, provider_id, model_id, telegram_api_key_encrypted, workspace_host_path, workspace_container_path, restart_policy FROM agents WHERE id = ? AND deleted_at IS NULL",
		id).Scan(&a.ID, &a.Name, &a.Slug, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.TelegramAPIKeyEncrypted, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy)

	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	containerID, err := h.docker.CreateAgentContainer(&a)
	if err != nil {
		database.AuditLogEntry(h.db, "system", "repair_agent", id, "Failed to repair agent", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.docker.StartContainer(containerID)

	h.db.Exec("UPDATE agents SET drift_state = 'in_sync', last_error = NULL, updated_at = ? WHERE id = ?", time.Now(), id)
	database.AuditLogEntry(h.db, "system", "repair_agent", id, "Repaired agent container", "success", nil)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) BackupAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var input struct {
		BackupType     string `json:"backup_type"`
		IncludeSecrets bool   `json:"include_secrets"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.BackupType = "full"
		input.IncludeSecrets = false
	}

	backupID, path, err := h.docker.CreateBackup(h.db, id, input.BackupType, input.IncludeSecrets)
	if err != nil {
		database.AuditLogEntry(h.db, "system", "backup_agent", id, "Failed to create backup", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "backup_agent", id, fmt.Sprintf("Created %s backup", input.BackupType), "success", map[string]interface{}{"backup_id": backupID, "path": path})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": backupID, "path": path})
}

func (h *Handler) RestoreAgent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var input struct {
		BackupID    string `json:"backup_id"`
		TargetAgent string `json:"target_agent"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.docker.RestoreBackup(h.db, input.BackupID, input.TargetAgent)
	if err != nil {
		database.AuditLogEntry(h.db, "system", "restore_agent", id, "Failed to restore backup", "error", nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	database.AuditLogEntry(h.db, "system", "restore_agent", id, fmt.Sprintf("Restored backup to %s", input.TargetAgent), "success", nil)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DownloadWorkspace(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	archivePath, err := h.docker.DownloadWorkspace(h.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		if archivePath != "" {
			// cleanup would happen here in production
		}
	}()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=workspace-%s.tar.gz", id))
	http.ServeFile(w, r, archivePath)
}

func (h *Handler) ValidatePath(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	if input.Path == "" || input.Path == "/" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}

func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}

func (h *Handler) TriggerReconcile(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := database.GetAuditLog(h.db, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(entries)
}

func sanitizeSlug(name string) string {
	slug := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			slug += string(c)
		}
	}
	return slug
}

var _ io.Closer = (*noopCloser)(nil)

type noopCloser struct{}

func (nc *noopCloser) Close() error { return nil }
