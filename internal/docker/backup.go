package docker

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/google/uuid"

	"clawdock/internal/models"
)

func (c *Client) CreateAgentContainer(a *models.Agent) (string, error) {
	containerName := fmt.Sprintf("openclaw-%s", a.Slug)

	config := &docker.Config{
		Image: a.ImageRepo + ":" + a.ImageTag,
		Env: []string{
			"PROVIDER_ID=" + a.ProviderID,
			"MODEL_ID=" + a.ModelID,
		},
		Cmd: []string{"/bin/bash", "-c", "cd /workspace && exec openclaw"},
	}

	hostConfig := &docker.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/workspace", a.WorkspaceHostPath),
		},
		CapAdd: []string{"NET_ADMIN"},
	}

	switch a.RestartPolicy {
	case "always":
		hostConfig.RestartPolicy = docker.AlwaysRestart()
	case "unless-stopped":
		hostConfig.RestartPolicy = docker.RestartPolicy{
			Name: "unless-stopped",
		}
	case "no":
		hostConfig.RestartPolicy = docker.NeverRestart()
	default:
		hostConfig.RestartPolicy = docker.RestartPolicy{
			Name: "unless-stopped",
		}
	}

	containerID, err := c.CreateContainer(config, hostConfig, containerName)
	if err != nil {
		return "", err
	}

	return containerID, nil
}

func (c *Client) CreateBackup(db *sql.DB, agentID, backupType string, includeSecrets bool) (string, string, error) {
	backupID := uuid.New().String()
	now := time.Now()

	var agent models.Agent
	var telegramKey sql.NullString
	err := db.QueryRow("SELECT id, name, slug, image_repo, image_tag, provider_id, model_id, workspace_host_path, workspace_container_path, restart_policy, telegram_api_key_encrypted FROM agents WHERE id = ?",
		agentID).Scan(&agent.ID, &agent.Name, &agent.Slug, &agent.ImageRepo, &agent.ImageTag, &agent.ProviderID, &agent.ModelID, &agent.WorkspaceHostPath, &agent.WorkspaceContainerPath, &agent.RestartPolicy, &telegramKey)

	if err != nil {
		return "", "", err
	}

	if telegramKey.Valid {
		agent.TelegramAPIKeyEncrypted = telegramKey.String
	}

	backupDir := "/var/lib/openclaw-manager/backups"
	os.MkdirAll(backupDir, 0755)

	archivePath := filepath.Join(backupDir, fmt.Sprintf("%s-%s.tar.gz", agent.Slug, now.Format("20060102-150405")))

	switch backupType {
	case "config_only":
		configData, _ := json.Marshal(agent)
		configPath := archivePath + ".json"
		os.WriteFile(configPath, configData, 0644)
		return backupID, configPath, nil

	case "workspace_only":
		cmd := exec.Command("tar", "czf", archivePath, "-C", filepath.Dir(agent.WorkspaceHostPath), filepath.Base(agent.WorkspaceHostPath))
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Run(); err != nil {
			return "", "", err
		}

	case "full":
		workspaceTar := archivePath + ".workspace.tar.gz"
		cmd := exec.Command("tar", "czf", workspaceTar, "-C", filepath.Dir(agent.WorkspaceHostPath), filepath.Base(agent.WorkspaceHostPath))
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Run(); err != nil {
			return "", "", err
		}

		configData, _ := json.Marshal(agent)
		configPath := archivePath + ".json"
		os.WriteFile(configPath, configData, 0644)
	}

	hash := sha256.New()
	if f, err := os.Open(archivePath); err == nil {
		io.Copy(hash, f)
		f.Close()
	}
	shaHex := fmt.Sprintf("%x", hash.Sum(nil))

	info, _ := os.Stat(archivePath)
	sizeBytes := int64(0)
	if info != nil {
		sizeBytes = info.Size()
	}

	_, err = db.Exec(`INSERT INTO backups (id, agent_id, backup_type, archive_path, sha256, size_bytes, includes_secrets, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		backupID, agentID, backupType, archivePath, shaHex, sizeBytes, includeSecrets, now)

	return backupID, archivePath, err
}

func (c *Client) RestoreBackup(db *sql.DB, backupID, targetAgentID string) error {
	var backup models.Backup
	var archivePath sql.NullString
	err := db.QueryRow("SELECT id, agent_id, backup_type, archive_path, includes_secrets FROM backups WHERE id = ?",
		backupID).Scan(&backup.ID, &backup.AgentID, &backup.BackupType, &archivePath, &backup.IncludesSecrets)

	if err != nil {
		return err
	}

	if !archivePath.Valid {
		return fmt.Errorf("backup archive path not found")
	}

	var agent models.Agent
	err = db.QueryRow("SELECT id, name, slug, workspace_host_path FROM agents WHERE id = ?",
		targetAgentID).Scan(&agent.ID, &agent.Name, &agent.Slug, &agent.WorkspaceHostPath)

	if err != nil {
		return err
	}

	if backup.BackupType == "workspace_only" || backup.BackupType == "full" {
		cmd := exec.Command("tar", "xzf", archivePath.String, "-C", filepath.Dir(agent.WorkspaceHostPath))
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	if backup.BackupType == "config_only" || backup.BackupType == "full" {
		configPath := archivePath.String + ".json"
		configData, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}

		var config models.Agent
		json.Unmarshal(configData, &config)

		db.Exec(`UPDATE agents SET image_tag = ?, provider_id = ?, model_id = ?, restart_policy = ?, updated_at = ? WHERE id = ?`,
			config.ImageTag, config.ProviderID, config.ModelID, config.RestartPolicy, time.Now(), targetAgentID)
	}

	return nil
}

func (c *Client) DownloadWorkspace(db *sql.DB, agentID string) (string, error) {
	var agent models.Agent
	err := db.QueryRow("SELECT id, name, slug, workspace_host_path FROM agents WHERE id = ?",
		agentID).Scan(&agent.ID, &agent.Name, &agent.Slug, &agent.WorkspaceHostPath)

	if err != nil {
		return "", err
	}

	downloadDir := "/var/lib/openclaw-manager/workspaces"
	os.MkdirAll(downloadDir, 0755)

	archivePath := filepath.Join(downloadDir, fmt.Sprintf("%s-%s.tar.gz", agent.Slug, time.Now().Format("20060102-150405")))

	cmd := exec.Command("tar", "czf", archivePath, "-C", filepath.Dir(agent.WorkspaceHostPath), filepath.Base(agent.WorkspaceHostPath))
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return archivePath, nil
}
