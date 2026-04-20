/**
 * Backups Component for Clawdock Dashboard
 */

const backups = (() => {
  let agentBackups = {};

  function init() {
    setupEventListeners();
  }

  function setupEventListeners() {
    // Backup form submission
    document.addEventListener('submit', async (e) => {
      if (e.target.id === 'backup-form') {
        e.preventDefault();
        await handleCreateBackup(e.target);
      }
      if (e.target.id === 'restore-form') {
        e.preventDefault();
        await handleRestore(e.target);
      }
    });

    // Backup modal actions
    document.addEventListener('click', async (e) => {
      if (e.target.matches('[data-action="create-backup"]')) {
        const agentId = state.get('selectedAgentId');
        if (agentId) {
          openBackupModal(agentId);
        }
      }
      if (e.target.matches('[data-action="cancel-backup"]')) {
        modals.close('backup-modal');
      }
      if (e.target.matches('[data-action="cancel-restore"]')) {
        modals.close('restore-modal');
      }
      if (e.target.matches('[data-action="download-backup"]')) {
        const backupId = e.target.dataset.backupId;
        const agentId = state.get('selectedAgentId');
        if (backupId && agentId) {
          downloadBackup(agentId, backupId);
        }
      }
      if (e.target.matches('[data-action="restore-backup"]')) {
        const backupId = e.target.dataset.backupId;
        const agentId = state.get('selectedAgentId');
        if (backupId && agentId) {
          openRestoreModal(agentId, backupId);
        }
      }
      if (e.target.matches('[data-action="delete-backup"]')) {
        const backupId = e.target.dataset.backupId;
        const agentId = state.get('selectedAgentId');
        if (backupId && agentId) {
          deleteBackup(agentId, backupId);
        }
      }
    });
  }

  function renderAgentBackups(agent) {
    const backups = agentBackups[agent.id] || [];

    return `
      <div class="toolbar">
        <div class="toolbar-left">
          <button class="btn btn-primary btn-sm" data-action="create-backup">
            📦 Create Backup
          </button>
        </div>
        <div class="toolbar-right">
          <span class="text-muted">${backups.length} backup${backups.length !== 1 ? 's' : ''}</span>
        </div>
      </div>

      ${backups.length === 0 ? `
        <div class="empty-state">
          <div class="empty-state-icon">📦</div>
          <div class="empty-state-title">No backups yet</div>
          <div class="empty-state-text">Create your first backup to preserve this agent's state</div>
        </div>
      ` : `
        <div class="table-container">
          <table class="table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Created</th>
                <th>Size</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${backups.map(backup => `
                <tr>
                  <td>
                    <span class="badge badge-${backup.backup_type === 'full' ? 'success' : backup.backup_type === 'config_only' ? 'info' : 'warning'}">
                      ${backup.backup_type.replace('_', ' ')}
                    </span>
                    ${backup.includes_secrets ? '<span class="badge badge-danger">Secrets</span>' : ''}
                  </td>
                  <td>${format.relative(backup.created_at)}</td>
                  <td>${backup.size_bytes ? format.bytes(backup.size_bytes) : '-'}</td>
                  <td>
                    <button class="btn btn-sm btn-secondary" data-action="restore-backup" data-backup-id="${backup.id}">
                      Restore
                    </button>
                    <button class="btn btn-sm btn-secondary" data-action="download-backup" data-backup-id="${backup.id}">
                      Download
                    </button>
                    <button class="btn btn-sm btn-danger" data-action="delete-backup" data-backup-id="${backup.id}">
                      Delete
                    </button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      `}
    `;
  }

  async function loadBackups(agentId) {
    if (!agentId) return;

    try {
      // Backup endpoint returns backup list or we'd need to query from the backend
      // For now, let's assume we store backups in state
      const backups = []; // This would come from an API call
      agentBackups[agentId] = backups;
    } catch (error) {
      console.error('Failed to load backups:', error);
    }
  }

  function openBackupModal(agentId) {
    const modal = document.getElementById('backup-modal');
    if (!modal) return;

    modal.querySelector('.modal-body').innerHTML = `
      <form id="backup-form" data-agent-id="${agentId}">
        <div class="form-group">
          <label class="form-label">Backup Type</label>
          <select name="backup_type" class="form-select" required>
            <option value="config_only">Config Only</option>
            <option value="workspace_only">Workspace Only</option>
            <option value="full" selected>Full (Config + Workspace)</option>
          </select>
          <p class="form-hint">Full backup includes both configuration and workspace data.</p>
        </div>

        <div class="form-group">
          <label class="toggle">
            <input type="checkbox" name="include_secrets" class="toggle-input">
            <span class="toggle-switch"></span>
            <span class="toggle-label">Include secrets in backup</span>
          </label>
          <p class="form-hint">Warning: Including secrets will store API keys in the backup.</p>
        </div>

        <div id="backup-form-errors"></div>
      </form>
    `;

    modals.open('backup-modal');
  }

  function openRestoreModal(agentId, backupId) {
    const modal = document.getElementById('restore-modal');
    if (!modal) return;

    modal.querySelector('.modal-body').innerHTML = `
      <form id="restore-form" data-agent-id="${agentId}" data-backup-id="${backupId}">
        <div class="form-group">
          <label class="form-label">Target Agent</label>
          <select name="target_agent_id" class="form-select" required>
            <option value="${agentId}">Current Agent (Overwrite)</option>
            ${state.get('agents').filter(a => a.id !== agentId).map(a => `
              <option value="${a.id}">${a.name}</option>
            `).join('')}
          </select>
          <p class="form-hint">Select which agent to restore this backup to.</p>
        </div>

        <div class="alert alert-warning">
          <strong>Warning:</strong> This will overwrite the target agent's configuration and/or workspace data.
        </div>
      </form>
    `;

    modals.open('restore-modal');
  }

  async function handleCreateBackup(form) {
    const agentId = form.dataset.agentId;
    const formData = new FormData(form);
    const data = {
      backup_type: formData.get('backup_type'),
      include_secrets: formData.has('include_secrets'),
    };

    // Validate
    const { isValid, errors } = validation.validateBackup(data);
    if (!isValid) {
      displayErrors('backup-form-errors', errors);
      return;
    }

    try {
      state.setLoading('action', true);
      await api.agents.createBackup(agentId, data);
      toasts.success('Backup created successfully');
      modals.close('backup-modal');
      await loadBackups(agentId);
      agentDetail.render(); // Refresh the backups tab
    } catch (error) {
      toasts.error(`Failed to create backup: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  async function handleRestore(form) {
    const agentId = form.dataset.agentId;
    const backupId = form.dataset.backupId;
    const formData = new FormData(form);
    const targetAgentId = formData.get('target_agent_id');

    try {
      state.setLoading('action', true);
      await api.agents.restoreBackup(agentId, {
        backup_id: backupId,
        target_agent: targetAgentId,
      });
      toasts.success('Backup restored successfully');
      modals.close('restore-modal');
      agentsList.refresh();
    } catch (error) {
      toasts.error(`Failed to restore backup: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  async function downloadBackup(agentId, backupId) {
    // Note: The API doesn't have a direct download endpoint for backups
    // We'd need to implement this on the backend or use workspace download
    toasts.info('Download feature requires backend support');
  }

  async function deleteBackup(agentId, backupId) {
    modals.confirm({
      title: 'Delete Backup',
      message: 'Are you sure you want to delete this backup? This action cannot be undone.',
      confirmText: 'Delete',
      type: 'danger',
      onConfirm: async () => {
        try {
          // Note: API doesn't have a delete backup endpoint yet
          toasts.info('Backup deletion requires backend support');
        } catch (error) {
          toasts.error(`Failed to delete backup: ${error.message}`);
        }
      },
    });
  }

  function displayErrors(containerId, errors) {
    const container = document.getElementById(containerId);
    if (!container) return;

    if (Object.keys(errors).length === 0) {
      container.innerHTML = '';
      return;
    }

    container.innerHTML = Object.entries(errors).map(([field, message]) => `
      <div class="form-error">${format.capitalize(field.replace('_', ' '))}: ${message}</div>
    `).join('');
  }

  return {
    init,
    renderAgentBackups,
    loadBackups,
    openBackupModal,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = backups;
}
