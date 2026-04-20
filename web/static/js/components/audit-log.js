/**
 * Audit Log Component for Clawdock Dashboard
 */

const auditLog = (() => {
  let entries = [];

  function init() {
    loadAuditLog();

    // Auto-refresh every 30 seconds
    setInterval(loadAuditLog, 30000);
  }

  async function loadAuditLog() {
    try {
      entries = await api.audit.list();
      render();
    } catch (error) {
      console.error('Failed to load audit log:', error);
    }
  }

  function render() {
    const container = document.getElementById('audit-content');
    if (!container) return;

    const filtered = getFilteredEntries();

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <div class="card-title">Audit Log</div>
          <button class="btn btn-sm btn-secondary" data-action="refresh-audit">🔄 Refresh</button>
        </div>
        <div class="card-body">
          ${renderFilters()}
          ${filtered.length === 0
            ? `
              <div class="empty-state">
                <div class="empty-state-icon">📋</div>
                <div class="empty-state-title">No audit entries</div>
                <div class="empty-state-text">Actions will be logged here</div>
              </div>
            `
            : `
              <div style="display: flex; flex-direction: column; gap: 0.5rem;">
                ${filtered.map(entry => renderAuditEntry(entry)).join('')}
              </div>
            `
          }
        </div>
      </div>
    `;
  }

  function renderFilters() {
    return `
      <div class="toolbar" style="margin-bottom: 1.5rem;">
        <div class="toolbar-left">
          <select class="form-select" data-filter="action" style="width: 150px;">
            <option value="all">All Actions</option>
            <option value="create_agent">Create</option>
            <option value="update_agent">Update</option>
            <option value="delete_agent">Delete</option>
            <option value="start_agent">Start</option>
            <option value="stop_agent">Stop</option>
            <option value="backup_agent">Backup</option>
          </select>
          <select class="form-select" data-filter="result" style="width: 120px;">
            <option value="all">All Results</option>
            <option value="success">Success</option>
            <option value="error">Error</option>
          </select>
        </div>
        <div class="toolbar-right">
          <span class="text-muted">${getFilteredEntries().length} entries</span>
        </div>
      </div>
    `;
  }

  function renderAuditEntry(entry) {
    const actionIcons = {
      create_agent: '➕',
      update_agent: '✏️',
      delete_agent: '🗑️',
      start_agent: '▶️',
      stop_agent: '⏸️',
      restart_agent: '🔄',
      recreate_agent: '♻️',
      repair_agent: '🔧',
      clone_agent: '📋',
      backup_agent: '💾',
      restore_agent: '📥',
    };

    const icon = actionIcons[entry.action] || '📝';
    const resultClass = entry.result === 'success' ? 'badge-success' : 'badge-danger';

    return `
      <div class="audit-entry" data-audit-id="${entry.id}">
        <div class="audit-icon" style="background: var(--bg-tertiary);">
          ${icon}
        </div>
        <div class="audit-content">
          <div class="audit-header">
            <span class="audit-action">${format.actionDisplay(entry.action)}</span>
            ${entry.agent_id
              ? `<span class="audit-agent">${getAgentName(entry.agent_id)}</span>`
              : ''
            }
            <span class="audit-time">${format.relative(entry.created_at)}</span>
          </div>
          <div class="audit-summary">${entry.summary}</div>
          <div style="margin-top: 0.5rem;">
            <span class="badge ${resultClass}">${entry.result}</span>
            <span class="text-muted" style="font-size: 0.75rem; margin-left: 0.5rem;">
              by ${entry.actor}
            </span>
          </div>
        </div>
      </div>
    `;
  }

  function getFilteredEntries() {
    const filters = state.get('filters.audit');

    if (!Array.isArray(entries)) {
      console.warn('Expected entries to be array, got:', typeof entries);
      return [];
    }

    return entries.filter(entry => {
      if (filters.action !== 'all' && !entry.action.includes(filters.action)) {
        return false;
      }
      if (filters.result !== 'all' && entry.result !== filters.result) {
        return false;
      }
      return true;
    });
  }

  function getAgentName(agentId) {
    const agents = state.get('agents');
    if (!Array.isArray(agents)) return agentId.slice(0, 8);
    const agent = agents.find(a => a.id === agentId);
    return agent ? agent.name : agentId.slice(0, 8);
  }

  return {
    init,
    render,
    loadAuditLog,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = auditLog;
}
