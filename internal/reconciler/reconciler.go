package reconciler

import (
	"database/sql"
	"log"
	"time"

	"clawdock/internal/docker"
	"clawdock/internal/models"
)

type Reconciler struct {
	db     *sql.DB
	docker *docker.Client
}

func New(db *sql.DB, docker *docker.Client) *Reconciler {
	return &Reconciler{db: db, docker: docker}
}

func (r *Reconciler) Run() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		if err := r.Reconcile(); err != nil {
			log.Printf("reconcile error: %v", err)
		}
	}
}

func (r *Reconciler) Reconcile() error {
	rows, err := r.db.Query("SELECT id, name, slug, enabled, image_repo, image_tag, provider_id, model_id, workspace_host_path, workspace_container_path, restart_policy, status_desired FROM agents WHERE deleted_at IS NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Enabled, &a.ImageRepo, &a.ImageTag, &a.ProviderID, &a.ModelID, &a.WorkspaceHostPath, &a.WorkspaceContainerPath, &a.RestartPolicy, &a.StatusDesired); err != nil {
			log.Printf("scan error: %v", err)
			continue
		}

		state, err := r.docker.InspectContainerState(a.ID)
		if err != nil {
			r.markDrift(&a, "missing_container", err.Error())
			continue
		}

		if !a.Enabled {
			if state.Running {
				r.docker.StopContainer(a.ID)
			}
			r.updateAgentState(&a, "stopped", "in_sync", nil)
			continue
		}

		switch a.StatusDesired {
		case "running":
			if !state.Running {
				if err := r.docker.StartContainer(a.ID); err != nil {
					r.markDrift(&a, "config_error", err.Error())
				} else {
					r.updateAgentState(&a, "running", "in_sync", nil)
				}
			} else {
				r.updateAgentState(&a, "running", "in_sync", nil)
			}
		case "stopped":
			if state.Running {
				r.docker.StopContainer(a.ID)
			}
			r.updateAgentState(&a, "stopped", "in_sync", nil)
		}
	}

	return nil
}

func (r *Reconciler) markDrift(a *models.Agent, driftState, errMsg string) {
	now := time.Now()
	r.db.Exec("UPDATE agents SET drift_state = ?, last_error = ?, last_reconciled_at = ? WHERE id = ?",
		driftState, errMsg, now, a.ID)
}

func (r *Reconciler) updateAgentState(a *models.Agent, statusActual, driftState string, errMsg *string) {
	now := time.Now()
	r.db.Exec("UPDATE agents SET status_actual = ?, drift_state = ?, last_error = ?, last_reconciled_at = ? WHERE id = ?",
		statusActual, driftState, errMsg, now, a.ID)
}
