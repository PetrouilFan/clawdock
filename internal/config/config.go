package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Security  SecurityConfig  `yaml:"security"`
	Paths     PathsConfig     `yaml:"paths"`
	Reconcile ReconcileConfig  `yaml:"reconcile"`
	Agents    AgentsConfig    `yaml:"agents"`
}

type ServerConfig struct {
	Host            string `yaml:"host"`
	Port            string `yaml:"port"`
	TLSCertFile     string `yaml:"tls_cert_file"`
	TLSKeyFile      string `yaml:"tls_key_file"`
	AllowedHosts    string `yaml:"allowed_hosts"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type SecurityConfig struct {
	RequireAuth   bool   `yaml:"require_auth"`
	SecretKey     string `yaml:"-"`
	SecretKeyFile string `yaml:"secret_key_file"`
	CSRFEnabled   bool   `yaml:"csrf_enabled"`
}

type PathsConfig struct {
	DataDir   string `yaml:"data_dir"`
	BackupDir string `yaml:"backup_dir"`
}

type ReconcileConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"`
}

type AgentsConfig struct {
	DefaultImageRepo           string `yaml:"default_image_repo"`
	DefaultRestartPolicy       string `yaml:"default_restart_policy"`
	DefaultWorkspaceContainerPath string `yaml:"default_workspace_container_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "11436"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/var/lib/openclaw-manager/manager.db"
	}
	if cfg.Agents.DefaultImageRepo == "" {
		cfg.Agents.DefaultImageRepo = "ghcr.io/openclaw/openclaw"
	}
	if cfg.Agents.DefaultRestartPolicy == "" {
		cfg.Agents.DefaultRestartPolicy = "unless-stopped"
	}
	if cfg.Agents.DefaultWorkspaceContainerPath == "" {
		cfg.Agents.DefaultWorkspaceContainerPath = "/workspace"
	}

	// Load secret key
	if cfg.Security.SecretKeyFile != "" {
		keyData, err := os.ReadFile(cfg.Security.SecretKeyFile)
		if err != nil {
			return nil, err
		}
		cfg.Security.SecretKey = strings.TrimSpace(string(keyData))
	}

	return &cfg, nil
}
