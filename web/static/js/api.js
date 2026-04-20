/**
 * API Client for Clawdock Dashboard
 * Handles all HTTP requests to the OpenClaw Manager backend
 */

const api = (() => {
  const baseUrl = '';

  async function request(endpoint, options = {}) {
    const url = `${baseUrl}${endpoint}`;
    const config = {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    };

    if (config.body && typeof config.body === 'object') {
      config.body = JSON.stringify(config.body);
    }

    try {
      const response = await fetch(url, config);

      if (!response.ok) {
        const error = await response.json().catch(() => ({ message: `HTTP ${response.status}` }));
        throw new Error(error.message || `Request failed: ${response.status}`);
      }

      if (response.status === 204) {
        return null;
      }

      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return await response.json();
      }

      return await response.text();
    } catch (error) {
      console.error(`API Error: ${endpoint}`, error);
      throw error;
    }
  }

  // Health & System
  const health = {
    check: () => request('/healthz'),
    ready: () => request('/readyz'),
    version: () => request('/version'),
  };

  // Agents
  const agents = {
    list: () => request('/api/agents'),
    get: (id) => request(`/api/agents/${id}`),
    create: (data) => request('/api/agents', { method: 'POST', body: data }),
    update: (id, data) => request(`/api/agents/${id}`, { method: 'PATCH', body: data }),
    delete: (id, mode = 'full') => request(`/api/agents/${id}?mode=${mode}`, { method: 'DELETE' }),

    // Lifecycle
    start: (id) => request(`/api/agents/${id}/start`, { method: 'POST' }),
    stop: (id) => request(`/api/agents/${id}/stop`, { method: 'POST' }),
    restart: (id) => request(`/api/agents/${id}/restart`, { method: 'POST' }),
    recreate: (id) => request(`/api/agents/${id}/recreate`, { method: 'POST' }),
    repair: (id) => request(`/api/agents/${id}/repair`, { method: 'POST' }),
    clone: (id, data) => request(`/api/agents/${id}/clone`, { method: 'POST', body: data }),

    // Utilities
    logs: (id) => request(`/api/agents/${id}/logs`),
    downloadWorkspace: (id) => `${baseUrl}/api/agents/${id}/workspace/download`,

    // Backup/Restore
    createBackup: (id, data) => request(`/api/agents/${id}/backup`, { method: 'POST', body: data }),
    restoreBackup: (id, data) => request(`/api/agents/${id}/restore`, { method: 'POST', body: data }),
  };

  // Providers
  const providers = {
    list: () => request('/api/providers'),
    models: (id) => request(`/api/providers/${id}/models`),
  };

  // Validation
  const validate = {
    path: (path) => request('/api/validate/path', { method: 'POST', body: { path } }),
    token: (token) => request('/api/validate/token', { method: 'POST', body: { token } }),
  };

  // Audit
  const audit = {
    list: () => request('/api/audit'),
  };

  // Reconcile
  const reconcile = {
    trigger: () => request('/api/reconcile', { method: 'POST' }),
  };

  return {
    health,
    agents,
    providers,
    validate,
    audit,
    reconcile,
    request,
  };
})();

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
  module.exports = api;
}
