package providers

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"clawdock/internal/database"
	"clawdock/internal/models"
	"clawdock/internal/security"
)

// Registry holds provider state and provides model resolution and discovery.
type Registry struct {
	db     *sql.DB
	secret []byte
}

// NewRegistry creates a new registry with DB and encryption secret.
func NewRegistry(db *sql.DB, secret []byte) *Registry {
	return &Registry{db: db, secret: secret}
}

// DiscoverAndUpsertModels fetches models from the provider's API and upserts them.
// Returns a summary with counts.
func (r *Registry) DiscoverAndUpsertModels(providerID string) (map[string]int, error) {
	// Load provider
	var p models.Provider
	var baseURL sql.NullString
	var apiKeyEncrypted sql.NullString
	err := r.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, api_key_encrypted
		FROM providers WHERE id = ?`, providerID).
		Scan(&p.ID, &p.DisplayName, &baseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &apiKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	if !p.Enabled || !p.SupportsModelDiscovery {
		return nil, fmt.Errorf("provider not enabled or discovery disabled")
	}
	if !baseURL.Valid || baseURL.String == "" {
		return nil, fmt.Errorf("provider has no base_url")
	}

	// Prepare API key if needed
	var apiKey string
	if p.AuthType == "api_key" || p.AuthType == "bearer" {
		if !apiKeyEncrypted.Valid || apiKeyEncrypted.String == "" {
			return nil, fmt.Errorf("provider has no api_key set")
		}
		decrypted, err := security.Decrypt(apiKeyEncrypted.String, r.secret)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt api key: %w", err)
		}
		apiKey = decrypted
	}

	// Build discoverer
	discoverer, err := NewDiscoverer(providerID, baseURL.String, p.AuthType, apiKey)
	if err != nil {
		return nil, fmt.Errorf("creating discoverer: %w", err)
	}

	modelKeys, err := discoverer.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Upsert each model
	added := 0
	updated := 0
	now := time.Now().Format(time.RFC3339)
	for _, key := range modelKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		// Check if exists
		var existingID string
		err := r.db.QueryRow("SELECT id FROM provider_models WHERE provider_id = ? AND model_key = ?", providerID, key).Scan(&existingID)
		if err == sql.ErrNoRows {
			// Insert new; use deterministic ID: providerID/model_key
			modelID := fmt.Sprintf("%s/%s", providerID, key)
			_, err = r.db.Exec(`INSERT INTO provider_models (id, provider_id, model_key, display_name, enabled, sort_order, created_at, updated_at, health_status)
			VALUES (?, ?, ?, ?, 1, 0, ?, ?, 'unknown')`,
				modelID, providerID, key, key, now, now)
			if err != nil {
				// Possibly duplicate, try update
				r.db.Exec(`UPDATE provider_models SET display_name = ?, enabled = 1, updated_at = ? WHERE provider_id = ? AND model_key = ?`,
					key, now, providerID, key)
				updated++
			} else {
				added++
			}
		} else if err == nil {
			// Update existing: ensure enabled and refresh display_name
			_, err = r.db.Exec(`UPDATE provider_models SET enabled = 1, display_name = ?, updated_at = ? WHERE id = ?`,
				key, now, existingID)
			if err == nil {
				updated++
			}
		}
		// ignore other errors
	}

	// Optionally disable models not in current list (sync exactly)
	// Build placeholders for IN clause
	if len(modelKeys) > 0 {
		placeholders := make([]string, len(modelKeys))
		args := make([]interface{}, len(modelKeys)+1)
		for i, k := range modelKeys {
			placeholders[i] = "?"
			args[i] = k
		}
		args[len(modelKeys)] = providerID
		query := fmt.Sprintf(`UPDATE provider_models SET enabled = 0, updated_at = ? 
			WHERE provider_id = ? AND model_key NOT IN (%s)`, strings.Join(placeholders, ","))
		if _, err := r.db.Exec(query, append([]interface{}{now}, args...)...); err != nil {
			log.Printf("warning: failed to disable stale models for provider %s: %v", providerID, err)
		}
	}

	// Update provider's updated_at
	if _, err := r.db.Exec(`UPDATE providers SET updated_at = ? WHERE id = ?`, now, providerID); err != nil {
		log.Printf("warning: failed to update provider updated_at: %v", err)
	}

	return map[string]int{"added": added, "updated": updated, "total": len(modelKeys)}, nil
}

// ResolveModel resolves a model key (or custom alias) to a provider and model.
func (r *Registry) ResolveModel(modelKey string) (*models.Provider, *models.ProviderModel, error) {
	// First check custom_models
	var custom struct {
		TargetProviderID string
		TargetModelKey   string
		Enabled          bool
	}
	err := r.db.QueryRow(`SELECT target_provider_id, target_model_key, enabled FROM custom_models WHERE id = ?`, modelKey).
		Scan(&custom.TargetProviderID, &custom.TargetModelKey, &custom.Enabled)
	if err == nil && custom.Enabled {
		// Resolve to underlying provider model
		var m models.ProviderModel
		err = r.db.QueryRow(`SELECT id, provider_id, model_key, display_name, enabled, sort_order FROM provider_models 
			WHERE provider_id = ? AND model_key = ? AND enabled = 1`,
			custom.TargetProviderID, custom.TargetModelKey).
			Scan(&m.ID, &m.ProviderID, &m.ModelKey, &m.DisplayName, &m.Enabled, &m.SortOrder)
		if err != nil {
			return nil, nil, fmt.Errorf("custom model target not found or disabled")
		}
		var p models.Provider
		err = r.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
			FROM providers WHERE id = ?`, custom.TargetProviderID).
			Scan(&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("provider for custom model not found")
		}
		if !p.Enabled {
			return nil, nil, fmt.Errorf("provider disabled")
		}
		return &p, &m, nil
	}
	if err != sql.ErrNoRows && err != nil {
		return nil, nil, err
	}

	// Not a custom alias; treat as direct provider model key
	var m models.ProviderModel
	err = r.db.QueryRow(`SELECT id, provider_id, model_key, display_name, enabled, sort_order FROM provider_models 
		WHERE model_key = ? AND enabled = 1`, modelKey).
		Scan(&m.ID, &m.ProviderID, &m.ModelKey, &m.DisplayName, &m.Enabled, &m.SortOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("model not found or disabled: %s", modelKey)
		}
		return nil, nil, err
	}

	var p models.Provider
	err = r.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, m.ProviderID).
		Scan(&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("provider not found: %w", err)
	}
	if !p.Enabled {
		return nil, nil, fmt.Errorf("provider disabled")
	}
	return &p, &m, nil
}

// GetDefaultModel returns the configured default model key.
func (r *Registry) GetDefaultModel() (string, error) {
	return database.GetSetting(r.db, "default_model")
}

// SetDefaultModel sets the global default model key.
func (r *Registry) SetDefaultModel(modelKey string) error {
	// Validate that the model exists and is enabled
	_, _, err := r.ResolveModel(modelKey)
	if err != nil {
		return fmt.Errorf("invalid default model: %w", err)
	}
	return database.SetSetting(r.db, "default_model", modelKey)
}

// IsChatProxyEnabled returns whether the chat proxy is turned on.
func (r *Registry) IsChatProxyEnabled() (bool, error) {
	val, err := database.GetSetting(r.db, "chat_proxy_enabled")
	if err != nil {
		return true, nil // default to true if setting missing
	}
	return val == "true", nil
}

// SetChatProxyEnabled toggles the proxy.
func (r *Registry) SetChatProxyEnabled(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return database.SetSetting(r.db, "chat_proxy_enabled", val)
}

// DecryptProviderKey returns the plaintext API key for a provider.
func (r *Registry) DecryptProviderKey(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	return security.Decrypt(encrypted, r.secret)
}

// GetProviderByID fetches a provider by ID.
func (r *Registry) GetProviderByID(id string) (*models.Provider, error) {
	var p models.Provider
	var baseURL sql.NullString
	var apiKeyEncrypted sql.NullString
	err := r.db.QueryRow(`SELECT id, display_name, base_url, api_key_encrypted, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, id).
		Scan(&p.ID, &p.DisplayName, &baseURL, &apiKeyEncrypted, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if baseURL.Valid {
		p.BaseURL = &baseURL.String
	}
	if apiKeyEncrypted.Valid {
		p.APIKeyEncrypted = apiKeyEncrypted.String
	}
	return &p, nil
}

// ListAllProviders returns all providers.
func (r *Registry) ListAllProviders() ([]models.Provider, error) {
	rows, err := r.db.Query(`SELECT id, display_name, base_url, api_key_encrypted, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []models.Provider
	for rows.Next() {
		var p models.Provider
		var baseURL sql.NullString
		var apiKeyEnc sql.NullString
		if err := rows.Scan(&p.ID, &p.DisplayName, &baseURL, &apiKeyEnc, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		if baseURL.Valid {
			p.BaseURL = &baseURL.String
		}
		if apiKeyEnc.Valid {
			p.APIKeyEncrypted = apiKeyEnc.String
		}
		providers = append(providers, p)
	}
	return providers, nil
}

// ListModelsForProvider returns all provider_models for a given provider.
func (r *Registry) ListModelsForProvider(providerID string) ([]models.ProviderModel, error) {
	rows, err := r.db.Query(`SELECT id, provider_id, model_key, display_name, enabled, sort_order, last_health_check, health_status
		FROM provider_models WHERE provider_id = ? ORDER BY sort_order`, providerID)
	if err != nil {
		return nil, err
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
	return result, nil
}

// UpdateProviderModelEnabled toggles a model's enabled flag.
func (r *Registry) UpdateProviderModelEnabled(modelID string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`UPDATE provider_models SET enabled = ?, updated_at = ? WHERE id = ?`, enabled, now, modelID)
	return err
}

// CheckProviderHealth updates the health_status of all models for a provider.
func (r *Registry) CheckProviderHealth(providerID string) error {
	// Fetch provider
	var p models.Provider
	var baseURL sql.NullString
	var apiKeyEncrypted sql.NullString
	err := r.db.QueryRow(`SELECT id, base_url, auth_type, api_key_encrypted FROM providers WHERE id = ?`, providerID).
		Scan(&p.ID, &baseURL, &p.AuthType, &apiKeyEncrypted)
	if err != nil {
		return err
	}
	if !baseURL.Valid || baseURL.String == "" {
		return fmt.Errorf("provider has no base_url")
	}
	var apiKey string
	if p.AuthType == "api_key" || p.AuthType == "bearer" {
		if apiKeyEncrypted.Valid && apiKeyEncrypted.String != "" {
			dec, err := security.Decrypt(apiKeyEncrypted.String, r.secret)
			if err != nil {
				return err
			}
			apiKey = dec
		} else {
			// No key available, mark offline? Skip
			return fmt.Errorf("no api key for provider")
		}
	}
	discoverer, err := NewDiscoverer(providerID, baseURL.String, p.AuthType, apiKey)
	if err != nil {
		return err
	}
	err = discoverer.HealthCheck()
	status := "offline"
	if err == nil {
		status = "online"
	}
	// Update all models of this provider
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = r.db.Exec(`UPDATE provider_models SET health_status = ?, last_health_check = ? WHERE provider_id = ?`, status, now, providerID)
	return nil
}

// CheckAllHealth iterates all enabled providers and updates model health.
func (r *Registry) CheckAllHealth() {
	providers, _ := r.listAllProvidersInternal()
	for _, p := range providers {
		if p.Enabled {
			_ = r.CheckProviderHealth(p.ID)
		}
	}
}

// internal helper to list providers without is_builtin (to avoid recursion)
func (r *Registry) listAllProvidersInternal() ([]models.Provider, error) {
	rows, err := r.db.Query(`SELECT id, display_name, base_url, api_key_encrypted, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.Provider
	for rows.Next() {
		var p models.Provider
		var baseURL sql.NullString
		var apiKeyEnc sql.NullString
		if err := rows.Scan(&p.ID, &p.DisplayName, &baseURL, &apiKeyEnc, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		if baseURL.Valid {
			p.BaseURL = &baseURL.String
		}
		if apiKeyEnc.Valid {
			p.APIKeyEncrypted = apiKeyEnc.String
		}
		list = append(list, p)
	}
	return list, nil
}

// GetModelByID fetches ProviderModel by its UUID.
func (r *Registry) GetModelByID(modelID string) (*models.ProviderModel, error) {
	var m models.ProviderModel
	var lastHealthCheck sql.NullString
	err := r.db.QueryRow(`SELECT id, provider_id, model_key, display_name, enabled, sort_order, last_health_check, health_status
		FROM provider_models WHERE id = ?`, modelID).
		Scan(&m.ID, &m.ProviderID, &m.ModelKey, &m.DisplayName, &m.Enabled, &m.SortOrder, &lastHealthCheck, &m.HealthStatus)
	if err != nil {
		return nil, err
	}
	if lastHealthCheck.Valid {
		t, _ := time.Parse(time.RFC3339, lastHealthCheck.String)
		m.LastHealthCheck = &t
	}
	return &m, nil
}

// Custom Model operations

// CreateCustomModel inserts a new alias.
func (r *Registry) CreateCustomModel(alias, targetProviderID, targetModelKey string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := r.db.Exec(`INSERT INTO custom_models (id, target_provider_id, target_model_key, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, alias, targetProviderID, targetModelKey, enabledInt, now, now)
	return err
}

// UpdateCustomModel updates an alias.
func (r *Registry) UpdateCustomModel(alias, targetProviderID, targetModelKey string, enabled bool) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Exec(`UPDATE custom_models SET target_provider_id = ?, target_model_key = ?, enabled = ?, updated_at = ?
		WHERE id = ?`, targetProviderID, targetModelKey, enabled, now, alias)
	return err
}

// DeleteCustomModel removes an alias.
func (r *Registry) DeleteCustomModel(alias string) error {
	_, err := r.db.Exec(`DELETE FROM custom_models WHERE id = ?`, alias)
	return err
}

// GetCustomModel fetches one alias.
func (r *Registry) GetCustomModel(alias string) (*models.CustomModel, error) {
	var c models.CustomModel
	err := r.db.QueryRow(`SELECT id, target_provider_id, target_model_key, enabled, created_at, updated_at
		FROM custom_models WHERE id = ?`, alias).
		Scan(&c.ID, &c.TargetProviderID, &c.TargetModelKey, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListAllCustomModels returns all custom model aliases.
func (r *Registry) ListAllCustomModels() ([]models.CustomModel, error) {
	rows, err := r.db.Query(`SELECT id, target_provider_id, target_model_key, enabled, created_at, updated_at
		FROM custom_models ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.CustomModel
	for rows.Next() {
		var c models.CustomModel
		if err := rows.Scan(&c.ID, &c.TargetProviderID, &c.TargetModelKey, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		list = append(list, c)
	}
	return list, nil
}

// GetCustomModelTarget returns the resolved provider+model for an alias, if enabled.
func (r *Registry) ResolveCustomModel(alias string) (*models.Provider, *models.ProviderModel, error) {
	c, err := r.GetCustomModel(alias)
	if err != nil {
		return nil, nil, err
	}
	if !c.Enabled {
		return nil, nil, fmt.Errorf("custom model disabled")
	}
	// Resolve target
	var m models.ProviderModel
	err = r.db.QueryRow(`SELECT id, provider_id, model_key, display_name, enabled, sort_order FROM provider_models
		WHERE provider_id = ? AND model_key = ? AND enabled = 1`, c.TargetProviderID, c.TargetModelKey).
		Scan(&m.ID, &m.ProviderID, &m.ModelKey, &m.DisplayName, &m.Enabled, &m.SortOrder)
	if err != nil {
		return nil, nil, fmt.Errorf("target model not found or disabled")
	}
	var p models.Provider
	err = r.db.QueryRow(`SELECT id, display_name, base_url, auth_type, enabled, supports_model_discovery, is_builtin, created_at, updated_at
		FROM providers WHERE id = ?`, c.TargetProviderID).
		Scan(&p.ID, &p.DisplayName, &p.BaseURL, &p.AuthType, &p.Enabled, &p.SupportsModelDiscovery, &p.IsBuiltin, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("provider not found")
	}
	if !p.Enabled {
		return nil, nil, fmt.Errorf("provider disabled")
	}
	return &p, &m, nil
}
