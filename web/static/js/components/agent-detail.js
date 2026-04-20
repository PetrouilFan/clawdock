/**
 * Agent Detail Component for Clawdock Dashboard
 */

const agentDetail = (() => {
  let container = null;
  let currentTab = 'overview';

  function init() {
    container = document.getElementById('agent-detail-content');

    // Listen for selected agent changes
    state.on('selectedAgentId', (agentId) => {
      if (agentId) {
        render();
      }
    });

    // Listen for agent updates
    state.on('agents', () => {
      if (state.get('selectedAgentId')) {
        render();
      }
    });

    setupEventListeners();
  }

  function render() {
    const agent = state.getSelectedAgent();
    if (!agent) return;

    const modal = document.getElementById('agent-detail-modal');
    if (!modal) return;

    // Update header
    const headerName = modal.querySelector('.agent-detail-name');
    const headerStatus = modal.querySelector('.agent-detail-status');

    if (headerName) headerName.textContent = agent.name;
    if (headerStatus) {
      const statusClass = agent.status_actual === 'running' ? 'badge-success' :
                         agent.last_error ? 'badge-danger' : 'badge-neutral';
      headerStatus.className = `badge ${statusClass}`;
      headerStatus.textContent = format.statusDisplay(agent.status_actual);
    }

    // Render tab content
    if (container) {
      container.innerHTML = renderTabContent(agent);
    }
  }

  function renderTabContent(agent) {
    switch (currentTab) {
      case 'overview':
        return renderOverview(agent);
      case 'config':
        return renderConfig(agent);
      case 'backups':
        return renderBackups(agent);
      case 'logs':
        return renderLogs(agent);
      case 'terminal':
        return renderTerminal(agent);
      default:
        return renderOverview(agent);
    }
  }

  function renderOverview(agent) {
    const driftInfo = agent.drift_state === 'drifted' ? `
      <div class="card mt-3">
        <div class="card-header">
          <div class="card-title">⚠️ Drift Detected</div>
        </div>
        <div class="card-body">
          <p>This agent's configuration has drifted from the desired state.</p>
          <button class="btn btn-primary" data-action="repair" data-agent-id="${agent.id}">Repair Agent</button>
        </div>
      </div>
    ` : '';

    const errorInfo = agent.last_error ? `
      <div class="card mt-3">
        <div class="card-header">
          <div class="card-title">❌ Error</div>
        </div>
        <div class="card-body">
          <pre class="logs-container" style="height: auto;">${dom.escapeHtml(agent.last_error)}</pre>
        </div>
      </div>
    ` : '';

    return `
      <div class="form-row">
        <div class="form-group">
          <label class="form-label">Agent ID</label>
          <input type="text" class="form-input" value="${agent.id}" disabled>
        </div>
        <div class="form-group">
          <label class="form-label">Slug</label>
          <input type="text" class="form-input" value="${agent.slug}" disabled>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label class="form-label">Provider</label>
          <input type="text" class="form-input" value="${format.providerDisplayName(agent.provider_id)}" disabled>
        </div>
        <div class="form-group">
          <label class="form-label">Model</label>
          <input type="text" class="form-input" value="${agent.model_id}" disabled>
        </div>
      </div>

      <div class="form-row">
        <div class="form-group">
          <label class="form-label">Image</label>
          <input type="text" class="form-input" value="${agent.image_repo}:${agent.image_tag}" disabled>
        </div>
        <div class="form-group">
          <label class="form-label">Restart Policy</label>
          <input type="text" class="form-input" value="${agent.restart_policy}" disabled>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">Workspace Host Path</label>
        <input type="text" class="form-input" value="${agent.workspace_host_path || ''}" disabled>
      </div>

      <div class="card mt-3">
        <div class="card-header">
          <div class="card-title">📊 Configuration Revision</div>
        </div>
        <div class="card-body">
          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Spec Version</label>
              <input type="text" class="form-input" value="${agent.spec_version}" disabled>
            </div>
            <div class="form-group">
              <label class="form-label">Config Revision</label>
              <input type="text" class="form-input" value="${agent.config_revision}" disabled>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label class="form-label">Created</label>
              <input type="text" class="form-input" value="${format.dateLong(agent.created_at)}" disabled>
            </div>
            <div class="form-group">
              <label class="form-label">Last Updated</label>
              <input type="text" class="form-input" value="${format.dateLong(agent.updated_at)}" disabled>
            </div>
          </div>
          ${agent.last_reconciled_at ? `
            <div class="form-group">
              <label class="form-label">Last Reconciled</label>
              <input type="text" class="form-input" value="${format.dateLong(agent.last_reconciled_at)}" disabled>
            </div>
          ` : ''}
        </div>
      </div>

      ${driftInfo}
      ${errorInfo}
    `;
  }

  function renderConfig(agent) {
    return agentForm.renderEditForm(agent);
  }

  function renderBackups(agent) {
    return backups.renderAgentBackups(agent);
  }

  function renderLogs(agent) {
    return `
      <div class="toolbar">
        <div class="toolbar-left">
          <button class="btn btn-secondary btn-sm" data-action="refresh-logs" data-agent-id="${agent.id}">🔄 Refresh</button>
          <button class="btn btn-secondary btn-sm" data-action="download-logs" data-agent-id="${agent.id}">📥 Download</button>
        </div>
        <div class="toolbar-right">
          <span class="text-muted">Last 500 lines</span>
        </div>
      </div>
      <div id="logs-content" class="logs-container">
        <div class="loading">
          <div class="spinner"></div>
        </div>
      </div>
    `;
  }

  function renderTerminal(agent) {
    return `
      <div class="terminal-container" data-agent-id="${agent.id}">
        <div class="terminal-header">
          <div class="terminal-title">
            💻 Terminal - ${dom.escapeHtml(agent.name)}
            <span id="terminal-status" class="terminal-status">Connecting...</span>
          </div>
          <div>
            <button class="btn btn-sm btn-secondary" data-action="reconnect-terminal">Reconnect</button>
            <button class="btn btn-sm btn-secondary" data-action="close-terminal">Close</button>
          </div>
        </div>
        <div id="terminal-container" class="terminal-body"></div>
      </div>
    `;
  }

  function setupEventListeners() {
    const modal = document.getElementById('agent-detail-modal');
    if (!modal) return;

    // Tab switching
    modal.querySelectorAll('.tab').forEach((tab) => {
      tab.addEventListener('click', () => {
        modal.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
        tab.classList.add('active');
        currentTab = tab.dataset.tab;
        render();

        // Load tab-specific data
        const agent = state.getSelectedAgent();
        if (agent) {
          if (currentTab === 'logs') {
            loadLogs(agent.id);
          } else if (currentTab === 'terminal') {
            terminal.connect(agent.id);
          } else if (currentTab === 'backups') {
            backups.loadBackups(agent.id);
          }
        }
      });
    });

    // Action buttons
    modal.addEventListener('click', async (e) => {
      const actionBtn = e.target.closest('[data-action]');
      if (!actionBtn) return;

      const action = actionBtn.dataset.action;
      const agentId = actionBtn.dataset.agentId;

      if (action === 'close') {
        modals.close('agent-detail-modal');
      } else if (action === 'repair') {
        await handleRepair(agentId);
      } else if (action === 'refresh-logs') {
        loadLogs(agentId);
      } else if (action === 'download-logs') {
        downloadLogs(agentId);
      } else if (action === 'reconnect-terminal') {
        terminal.connect(agentId);
      } else if (action === 'close-terminal') {
        terminal.disconnect();
      }
    });
  }

  async function handleRepair(agentId) {
    try {
      state.setLoading('action', true);
      await api.agents.repair(agentId);
      toasts.success('Agent repaired successfully');
      agentsList.refresh();
    } catch (error) {
      toasts.error(`Failed to repair agent: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  async function loadLogs(agentId) {
    const logsContainer = document.getElementById('logs-content');
    if (!logsContainer) return;

    logsContainer.innerHTML = `
      <div class="loading">
        <div class="spinner"></div>
      </div>
    `;

    try {
      const logs = await api.agents.logs(agentId);
      logsContainer.innerHTML = logs.split('\n').map((line, i) => {
        if (!line.trim()) return '';
        return `<div class="log-line" data-line="${i}">
          <span class="log-timestamp">${i + 1}</span>  ${dom.escapeHtml(line)}
        </div>`;
      }).join('');
      logsContainer.scrollTop = logsContainer.scrollHeight;
    } catch (error) {
      logsContainer.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-text">Failed to load logs: ${dom.escapeHtml(error.message)}</div>
        </div>
      `;
    }
  }

  function downloadLogs(agentId) {
    const agent = state.get('agents').find((a) => a.id === agentId);
    if (!agent) return;

    api.agents.logs(agentId).then((logs) => {
      const blob = new Blob([logs], { type: 'text/plain' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${agent.slug}-logs.txt`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    });
  }

  return {
    init,
    render,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = agentDetail;
}
