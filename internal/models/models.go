package models

import "time"

type Agent struct {
	ID                      string     `json:"id"`
	Name                    string     `json:"name"`
	Slug                    string     `json:"slug"`
	Enabled                 bool       `json:"enabled"`
	ImageRepo               string     `json:"image_repo"`
	ImageTag                string     `json:"image_tag"`
	ProviderID              string     `json:"provider_id"`
	ModelID                 string     `json:"model_id"`
	TelegramAPIKeyEncrypted string     `json:"-"`
	WorkspaceHostPath       string     `json:"workspace_host_path"`
	WorkspaceContainerPath  string     `json:"workspace_container_path"`
	ExtraEnvJSON            string     `json:"extra_env_json"`
	CommandOverrideJSON     string     `json:"command_override_json"`
	RestartPolicy           string     `json:"restart_policy"`
	StatusDesired           string     `json:"status_desired"`
	StatusActual            string     `json:"status_actual"`
	DriftState              string     `json:"drift_state"`
	LastError               *string    `json:"last_error,omitempty"`
	LastReconciledAt        *time.Time `json:"last_reconciled_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	DeletedAt               *time.Time `json:"deleted_at,omitempty"`
	SpecVersion             int        `json:"spec_version"`
	ConfigRevision          int        `json:"config_revision"`
}

type Provider struct {
	ID                     string    `json:"id"`
	DisplayName            string    `json:"display_name"`
	BaseURL                *string   `json:"base_url,omitempty"`
	APIKeyEncrypted        string    `json:"-"`
	AuthType               string    `json:"auth_type"`
	Enabled                bool      `json:"enabled"`
	SupportsModelDiscovery bool      `json:"supports_model_discovery"`
	IsBuiltin              bool      `json:"is_builtin"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ProviderModel struct {
	ID              string     `json:"id"`
	ProviderID      string     `json:"provider_id"`
	ModelKey        string     `json:"model_key"`
	DisplayName     string     `json:"display_name"`
	Enabled         bool       `json:"enabled"`
	SortOrder       int        `json:"sort_order"`
	MetadataJSON    string     `json:"metadata_json,omitempty"`
	LastHealthCheck *time.Time `json:"last_health_check,omitempty"`
	HealthStatus    string     `json:"health_status,omitempty"`
}

type Backup struct {
	ID              string    `json:"id"`
	AgentID         string    `json:"agent_id"`
	BackupType      string    `json:"backup_type"`
	ArchivePath     *string   `json:"archive_path,omitempty"`
	SHA256          *string   `json:"sha256,omitempty"`
	SizeBytes       *int64    `json:"size_bytes,omitempty"`
	IncludesSecrets bool      `json:"includes_secrets"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       *string   `json:"created_by,omitempty"`
}

type AuditLogEntry struct {
	ID          string    `json:"id"`
	Actor       string    `json:"actor"`
	Action      string    `json:"action"`
	AgentID     *string   `json:"agent_id,omitempty"`
	Summary     string    `json:"summary"`
	PayloadJSON *string   `json:"payload_json,omitempty"`
	Result      string    `json:"result"`
	CreatedAt   time.Time `json:"created_at"`
}

type ReconcilerRun struct {
	ID          string     `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Summary     *string    `json:"summary,omitempty"`
	Result      string     `json:"result"`
	DetailsJSON *string    `json:"details_json,omitempty"`
}

type CustomModel struct {
	ID               string    `json:"id"`
	TargetProviderID string    `json:"target_provider_id"`
	TargetModelKey   string    `json:"target_model_key"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// Not exposing updated_at in JSON for read; used internally
}
