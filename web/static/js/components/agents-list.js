/**
 * Agents List Component for Clawdock Dashboard
 */

const agentsList = (() => {
  let container = null;

  function init() {
    container = document.getElementById('agents-list');
    if (!container) return;

    setupEventListeners();

    // Listen for state changes
    state.on('agents', render);
    state.on('filters.agents', render);
    state.on('loading.agents', updateLoadingState);

    // Initial load
    loadAgents();

    // Auto-refresh every 5 seconds
    setInterval(loadAgents, 5000);
  }

  async function loadAgents() {
    if (state.get('loading.agents')) return;

    state.setLoading('agents', true);
    try {
      const agents = await api.agents.list();
      state.set('agents', agents);
      state.clearError('agents');
    } catch (error) {
      state.setError('agents', error.message);
      toasts.error('Failed to load agents: ' + error.message);
    } finally {
      state.setLoading('agents', false);
    }
  }

  function render() {
    if (!container) return;

    const agents = state.getFilteredAgents();
    const stats = state.getAgentStats();

    // Render stats
    const statsContainer = document.getElementById('stats-grid');
    if (statsContainer) {
      statsContainer.innerHTML = `
        <div class="stat-card">
          <div class="stat-icon blue">🤖</div>
          <div class="stat-content">
            <div class="stat-value">${stats.total}</div>
            <div class="stat-label">Total Agents</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon green">▶</div>
          <div class="stat-content">
            <div class="stat-value">${stats.running}</div>
            <div class="stat-label">Running</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon yellow">⚠</div>
          <div class="stat-content">
            <div class="stat-value">${stats.drifted}</div>
            <div class="stat-label">Drifted</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon red">✕</div>
          <div class="stat-content">
            <div class="stat-value">${stats.errors}</div>
            <div class="stat-label">Errors</div>
          </div>
        </div>
      `;
    }

    // Render agents
    if (agents.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🦀</div>
          <div class="empty-state-title">No agents found</div>
          <div class="empty-state-text">Create your first agent to get started</div>
          <button class="btn btn-primary" onclick="modals.openAgentCreate()">
            Create Agent
          </button>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="agents-grid">
        ${agents.map(agent => renderAgentCard(agent)).join('')}
      </div>
    `;
  }

  function renderAgentCard(agent) {
    const statusClass = agent.status_actual === 'running' ? 'running' :
                       agent.last_error ? 'error' : 'stopped';
    const driftBadge = agent.drift_state === 'drifted' ?
      `<span class="badge badge-warning">Drifted</span>` : '';

    return `
      <div class="agent-card" data-agent-id="${agent.id}">
        <div class="agent-card-header">
          <div>
            <div class="agent-name">${dom.escapeHtml(agent.name)}</div>
            <div class="agent-slug">${agent.slug}</div>
          </div>
          <div class="agent-status ${statusClass}">
            <span class="agent-status-dot"></span>
            <span>${format.statusDisplay(agent.status_actual)}</span>
          </div>
        </div>
        <div class="agent-details">
          <div class="agent-detail">
            <span class="agent-detail-label">Provider</span>
            <span class="agent-detail-value">${format.providerDisplayName(agent.provider_id)}</span>
          </div>
          <div class="agent-detail">
            <span class="agent-detail-label">Model</span>
            <span class="agent-detail-value">${format.truncate(agent.model_id, 20)}</span>
          </div>
          <div class="agent-detail">
            <span class="agent-detail-label">Image</span>
            <span class="agent-detail-value">${agent.image_tag}</span>
          </div>
          <div class="agent-detail">
            <span class="agent-detail-label">Updated</span>
            <span class="agent-detail-value" title="${format.dateLong(agent.updated_at)}">${format.relative(agent.updated_at)}</span>
          </div>
        </div>
        ${driftBadge}
        <div class="agent-actions">
          ${renderActionButtons(agent)}
        </div>
      </div>
    `;
  }

  function renderActionButtons(agent) {
    const isRunning = agent.status_actual === 'running';

    return `
      ${isRunning
        ? `<button class="btn btn-sm btn-secondary" data-action="stop" data-agent-id="${agent.id}" title="Stop">⏸</button>`
        : `<button class="btn btn-sm btn-success" data-action="start" data-agent-id="${agent.id}" title="Start">▶</button>`
      }
      <button class="btn btn-sm btn-secondary" data-action="terminal" data-agent-id="${agent.id}" title="Terminal" ${!isRunning ? 'disabled' : ''}>💻</button>
      <button class="btn btn-sm btn-secondary" data-action="edit" data-agent-id="${agent.id}" title="Edit">✏️</button>
      <button class="btn btn-sm btn-secondary" data-action="backup" data-agent-id="${agent.id}" title="Backup">💾</button>
      <button class="btn btn-sm btn-danger" data-action="delete" data-agent-id="${agent.id}" title="Delete">🗑️</button>
    `;
  }

  function setupEventListeners() {
    if (!container) return;

    // Agent card click
    container.addEventListener('click', (e) => {
      const card = e.target.closest('.agent-card');
      if (!card) return;

      const agentId = card.dataset.agentId;
      const actionBtn = e.target.closest('[data-action]');

      if (actionBtn) {
        e.stopPropagation();
        const action = actionBtn.dataset.action;
        handleAction(action, agentId);
      } else {
        modals.openAgentDetail(agentId);
      }
    });

    // Toolbar buttons
    const createBtn = document.getElementById('create-agent-btn');
    if (createBtn) {
      createBtn.addEventListener('click', () => modals.openAgentCreate());
    }

    const searchInput = document.getElementById('agent-search');
    if (searchInput) {
      searchInput.addEventListener('input', (e) => {
        state.setAgentFilter('search', e.target.value);
      });
    }

    const statusFilter = document.getElementById('agent-status-filter');
    if (statusFilter) {
      statusFilter.addEventListener('change', (e) => {
        state.setAgentFilter('status', e.target.value);
      });
    }
  }

  async function handleAction(action, agentId) {
    state.setLoading('action', true);

    try {
      switch (action) {
        case 'start':
          await api.agents.start(agentId);
          toasts.success('Agent started successfully');
          break;
        case 'stop':
          await api.agents.stop(agentId);
          toasts.success('Agent stopped successfully');
          break;
        case 'terminal':
          modals.openTerminal(agentId);
          return;
        case 'edit':
          modals.openAgentDetail(agentId);
          return;
        case 'backup':
          modals.openBackup(agentId);
          return;
        case 'delete':
          const agent = state.get('agents').find(a => a.id === agentId);
          modals.confirm({
            title: 'Delete Agent',
            message: `Are you sure you want to delete "${agent?.name || agentId}"? This action cannot be undone.`,
            confirmText: 'Delete',
            type: 'danger',
            onConfirm: async () => {
              await api.agents.delete(agentId, 'full');
              toasts.success('Agent deleted successfully');
              loadAgents();
            }
          });
          return;
      }

      // Refresh agents after action
      loadAgents();
    } catch (error) {
      toasts.error(`Failed to ${action} agent: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  function updateLoadingState(isLoading) {
    const loadingEl = document.getElementById('agents-loading');
    if (loadingEl) {
      loadingEl.classList.toggle('hidden', !isLoading);
    }
  }

  return {
    init,
    loadAgents,
    refresh: loadAgents
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = agentsList;
}
