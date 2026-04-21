/**
 * State Management for Clawdock Dashboard
 * Simple Pub/Sub pattern for reactive state updates
 */

const state = (() => {
  const store = {
    // Data
    agents: [],
    providers: [],
    providerModelsCache: {}, // provider_id -> models array
    customModels: [],
    settings: {
      default_model: '',
      chat_proxy_enabled: true,
    },
    auditLog: [],
    backups: [],
    systemStatus: null,

    // UI State
    currentView: 'agents',
    selectedAgentId: null,
    selectedProviderId: null,
    sidebarOpen: true,

    // Filters
    filters: {
      agents: {
        search: '',
        status: 'all',
        drift: 'all',
        provider: 'all',
      },
      audit: {
        action: 'all',
        agent: 'all',
        result: 'all',
      },
    },

    // Loading states
    loading: {
      agents: false,
      providers: false,
      customModels: false,
      settings: false,
      audit: false,
      action: false,
    },

    // Errors
    errors: {},

    // Modals
    modals: {
      createAgent: false,
      editAgent: false,
      agentDetail: false,
      terminal: false,
      backup: false,
      restore: false,
      confirm: false,
      createProvider: false,
      editProvider: false,
      createCustomModel: false,
      editCustomModel: false,
      settings: false,
    },

    // Toast notifications
    toasts: [],
  };

  const listeners = new Map();

  function on(event, callback) {
    if (!listeners.has(event)) {
      listeners.set(event, new Set());
    }
    listeners.get(event).add(callback);

    return () => {
      listeners.get(event).delete(callback);
    };
  }

  function emit(event, data) {
    if (listeners.has(event)) {
      listeners.get(event).forEach((callback) => {
        try {
          callback(data);
        } catch (error) {
          console.error(`Error in event listener for ${event}:`, error);
        }
      });
    }
  }

  function get(key) {
    if (key.includes('.')) {
      const parts = key.split('.');
      let value = store;
      for (const part of parts) {
        value = value?.[part];
      }
      return value;
    }
    return store[key];
  }

  function set(key, value) {
    if (key.includes('.')) {
      const parts = key.split('.');
      let target = store;
      for (let i = 0; i < parts.length - 1; i++) {
        target = target[parts[i]];
      }
      target[parts[parts.length - 1]] = value;
    } else {
      store[key] = value;
    }
    emit(key, value);
  }

  function update(key, updater) {
    const current = get(key);
    if (Array.isArray(current)) {
      set(key, updater([...current]));
    } else if (typeof current === 'object' && current !== null) {
      set(key, updater({ ...current }));
    } else {
      set(key, updater(current));
    }
  }

  // Helper methods for common operations
  function setLoading(key, value) {
    update('loading', (loading) => ({ ...loading, [key]: value }));
  }

  function setError(key, error) {
    update('errors', (errors) => ({ ...errors, [key]: error }));
  }

  function clearError(key) {
    update('errors', (errors) => {
      const newErrors = { ...errors };
      delete newErrors[key];
      return newErrors;
    });
  }

  function openModal(name) {
    update('modals', (modals) => ({ ...modals, [name]: true }));
  }

  function closeModal(name) {
    update('modals', (modals) => ({ ...modals, [name]: false }));
  }

  function addToast(message, type = 'info', duration = 5000) {
    const toast = {
      id: Date.now() + Math.random(),
      message,
      type,
      duration,
    };
    update('toasts', (toasts) => [...toasts, toast]);

    if (duration > 0) {
      setTimeout(() => removeToast(toast.id), duration);
    }

    return toast.id;
  }

  function removeToast(id) {
    update('toasts', (toasts) => toasts.filter((t) => t.id !== id));
  }

  function setView(view) {
    set('currentView', view);
  }

  function selectAgent(id) {
    set('selectedAgentId', id);
    if (id) {
      openModal('agentDetail');
    } else {
      closeModal('agentDetail');
    }
  }

  function setAgentFilter(key, value) {
    update('filters.agents', (filters) => ({ ...filters, [key]: value }));
  }

  function resetAgentFilters() {
    set('filters.agents', {
      search: '',
      status: 'all',
      drift: 'all',
      provider: 'all',
    });
  }

  // Computed values
  function getFilteredAgents() {
    const agents = get('agents');
    const filters = get('filters.agents');

    if (!Array.isArray(agents)) {
      console.warn('Expected agents to be array, got:', typeof agents, agents);
      return [];
    }

    return agents.filter((agent) => {
      // Search filter
      if (filters.search) {
        const search = filters.search.toLowerCase();
        const match =
          agent.name?.toLowerCase().includes(search) ||
          agent.slug?.toLowerCase().includes(search) ||
          agent.provider_id?.toLowerCase().includes(search) ||
          agent.model_id?.toLowerCase().includes(search);
        if (!match) return false;
      }

      // Status filter
      if (filters.status !== 'all' && agent.status_actual !== filters.status) {
        return false;
      }

      // Drift filter
      if (filters.drift !== 'all' && agent.drift_state !== filters.drift) {
        return false;
      }

      // Provider filter
      if (filters.provider !== 'all' && agent.provider_id !== filters.provider) {
        return false;
      }

      return true;
    });
  }

  function getAgentStats() {
    const agents = get('agents');
    const total = agents.length;
    const running = agents.filter((a) => a.status_actual === 'running').length;
    const drifted = agents.filter((a) => a.drift_state === 'drifted').length;
    const errors = agents.filter((a) => a.last_error).length;
    const stopped = agents.filter((a) => a.status_actual === 'stopped').length;

    return { total, running, drifted, errors, stopped };
  }

  function getSelectedAgent() {
    const id = get('selectedAgentId');
    if (!id) return null;
    return get('agents').find((a) => a.id === id) || null;
  }

  // Provider models cache
  function setProviderModelsCache(providerId, models) {
    update('providerModelsCache', (cache) => ({ ...cache, [providerId]: models }));
  }
  function getProviderModelsCache(providerId) {
    return get('providerModelsCache')[providerId] || [];
  }

  // Custom models
  function setCustomModels(models) {
    set('customModels', models);
  }
  function getCustomModels() {
    return get('customModels');
  }

  // Settings
  function setSettings(settings) {
    set('settings', { ...get('settings'), ...settings });
  }
  function getSetting(key) {
    return get('settings')[key];
  }

  return {
    // Core
    get,
    set,
    update,
    on,
    emit,

    // Helpers
    setLoading,
    setError,
    clearError,
    openModal,
    closeModal,
    addToast,
    removeToast,
    setView,
    selectAgent,
    setAgentFilter,
    resetAgentFilters,

    // Computed
    getFilteredAgents,
    getAgentStats,
    getSelectedAgent,

    // Extended
    setProviderModelsCache,
    getProviderModelsCache,
    setCustomModels,
    getCustomModels,
    setSettings,
    getSetting,
  };
})();

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
  module.exports = state;
}
