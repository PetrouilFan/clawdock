/**
 * Providers Component for Clawdock Dashboard
 */

const providers = (() => {
  let providersList = [];
  let healthPollingInterval = null;

  function init() {
    loadProviders();
    setupEventListeners();
    // Start health status polling every 30 seconds
    healthPollingInterval = setInterval(refreshHealthStatus, 30000);
  }

  // Stop polling when component is destroyed (optional cleanup)
  function destroy() {
    if (healthPollingInterval) clearInterval(healthPollingInterval);
  }

  async function loadProviders() {
    try {
      providersList = await api.providers.list();
      state.set('providers', providersList);
      render();

      // Load models for each provider in parallel
      await Promise.all(providersList.map((p) => loadModels(p.id)));
    } catch (error) {
      console.error('Failed to load providers:', error);
      state.addToast('Failed to load providers', 'error');
    } finally {
      state.setLoading('providers', false);
    }
  }

  async function loadModels(providerId) {
    if (state.getProviderModelsCache(providerId).length > 0) return;

    try {
      const models = await api.providers.listModels(providerId);
      state.setProviderModelsCache(providerId, models);
    } catch (error) {
      console.error('Failed to load models:', error);
    }
  }

  async function refreshHealthStatus() {
    try {
      const statuses = await api.models.status();
      // statuses is array of {model_key, provider_id, health_status}
      // Update the cache for each model
      statuses.forEach((entry) => {
        // Find all providers' caches and update matching model_key
        const pCache = state.get('providerModelsCache');
        Object.keys(pCache).forEach((pid) => {
          const models = pCache[pid] || [];
          const updated = models.map((m) => {
            if (m.model_key === entry.model_key && pid === entry.provider_id) {
              return { ...m, health_status: entry.health_status };
            }
            return m;
          });
          state.setProviderModelsCache(pid, updated);
        });
      });
      // Re-render if providers view is visible
      render();
    } catch (error) {
      console.error('Failed to refresh health status:', error);
    }
  }


  function setupEventListeners() {
    // Create provider button
    document.addEventListener('click', (e) => {
      if (e.target.matches('#btn-create-provider')) {
        openCreateModal();
      }
      if (e.target.matches('.provider-edit-btn')) {
        const id = e.target.closest('[data-provider-id]').dataset.providerId;
        openEditModal(id);
      }
      if (e.target.matches('.provider-delete-btn')) {
        const id = e.target.closest('[data-provider-id]').dataset.providerId;
        confirmDelete(id);
      }
      if (e.target.matches('.provider-refresh-btn')) {
        const id = e.target.closest('[data-provider-id]').dataset.providerId;
        refreshModels(id);
      }
      if (e.target.matches('.model-toggle')) {
        const checkbox = e.target;
        const modelId = checkbox.dataset.modelId;
        const enabled = checkbox.checked;
        toggleModel(modelId, enabled, checkbox);
      }
      // Cancel button in provider modal
      if (e.target.matches('#provider-modal [data-action="close-modal"]')) {
        closeModal();
      }
    });

    // Provider form submission
    document.addEventListener('submit', (e) => {
      if (e.target && e.target.id === 'provider-form') {
        e.preventDefault();
        saveProvider(e);
      }
    });

    // Auth type change in provider form to show/hide API key field
    document.addEventListener('change', (e) => {
      if (e.target.id === 'provider-auth-type') {
        const apiKeyGroup = document.getElementById('provider-api-key-group');
        if (apiKeyGroup) {
          apiKeyGroup.style.display = e.target.value === 'none' ? 'none' : 'block';
        }
      }
    });
  }

  function render() {
    const container = document.getElementById('providers-content');
    if (!container) return;

    if (!Array.isArray(providersList) || providersList.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🔌</div>
          <div class="empty-state-title">No providers available</div>
          <div class="empty-state-text">Create a provider to get started.</div>
          <button class="btn btn-primary" id="btn-create-provider">+ Create Provider</button>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <div class="card-title">AI Providers</div>
          <div>
            <span class="text-muted">${providersList.length} provider${providersList.length !== 1 ? 's' : ''}</span>
            <button class="btn btn-sm btn-primary" id="btn-create-provider">+ Create Provider</button>
          </div>
        </div>
        <div class="card-body">
          <div class="providers-grid" style="display: grid; gap: 1.5rem;">
            ${providersList.map(renderProviderCard).join('')}
          </div>
        </div>
      </div>
    `;
  }

  function renderProviderCard(provider) {
    const models = state.getProviderModelsCache(provider.id) || [];
    const enabledModels = models.filter((m) => m.enabled);

    return `
      <div class="provider-card" data-provider-id="${provider.id}">
        <div class="provider-header">
          <div class="provider-info">
            <div class="provider-icon">${getProviderIcon(provider.id)}</div>
            <div>
              <div class="provider-name">
                ${provider.display_name}
                ${provider.is_builtin ? '<span class="badge badge-neutral" style="margin-left:0.5rem">Built-in</span>' : ''}
              </div>
              <div class="provider-meta">
                ${provider.auth_type !== 'none' ? '🔐 Authentication required' : 'No authentication'}
                ${provider.base_url ? `• ${provider.base_url}` : ''}
              </div>
            </div>
          </div>
          <div class="provider-status">
            ${provider.enabled
              ? '<span class="badge badge-success">Enabled</span>'
              : '<span class="badge badge-neutral">Disabled</span>'}
          </div>
        </div>

        <div class="provider-models">
          <div style="padding: 1rem 1.5rem; border-bottom: 1px solid var(--border-primary); display:flex; align-items:center; justify-content:space-between;">
            <span class="text-muted" style="font-size: 0.875rem;">
              ${models.length} model${models.length !== 1 ? 's' : ''} (${enabledModels.length} enabled)
            </span>
            <div style="display: flex; gap: 0.5rem;">
              <button class="btn btn-sm btn-ghost provider-refresh-btn" data-provider-id="${provider.id}" title="Refresh Models">
                🔄
              </button>
              <button class="btn btn-sm btn-ghost provider-edit-btn" data-provider-id="${provider.id}" title="Edit Provider">
                ✏️
              </button>
              ${!provider.is_builtin ? `
                <button class="btn btn-sm btn-ghost provider-delete-btn" data-provider-id="${provider.id}" title="Delete Provider" style="color: var(--danger);">
                  🗑️
                </button>
              ` : ''}
            </div>
          </div>
          <div class="models-list">
            ${models.length === 0
              ? '<div style="padding: 1rem 1.5rem; color: var(--text-muted); font-size: 0.875rem;">No models available. Click refresh to discover.</div>'
              : models.map((model) => `
                <div class="model-row" style="padding: 0.75rem 1.5rem; display:flex; align-items:center; justify-content:space-between; border-bottom:1px solid var(--border-secondary);">
                  <div>
                    <div class="model-name">${model.display_name}</div>
                    <div class="model-key" style="font-size:0.75rem; color:var(--text-muted);">${model.model_key}</div>
                  </div>
                  <div style="display:flex; align-items:center; gap:0.5rem;">
                    <label class="toggle-switch">
                      <input type="checkbox" class="model-toggle" data-model-id="${model.id}" ${model.enabled ? 'checked' : ''}>
                      <span class="slider"></span>
                    </label>
                    <span class="badge ${model.health_status === 'online' ? 'badge-success' : model.health_status === 'offline' ? 'badge-error' : 'badge-neutral'}">${model.health_status || 'unknown'}</span>
                  </div>
                </div>
              `).join('')
            }
          </div>
        </div>
      </div>
    `;
  }

  function getProviderIcon(providerId) {
    const icons = {
      openai: '🅞',
      anthropic: '🅐',
      google: '🅖',
      ollama: '🅾️',
      openrouter: '🔲',
      custom: '⚙️',
    };
    return icons[providerId] || '🤖';
  }

  // Modal management
  function openCreateModal() {
    const modal = document.getElementById('provider-modal');
    if (!modal) return;
    modal.querySelector('form').reset();
    modal.dataset.mode = 'create';
    modal.dataset.providerId = '';
    modal.querySelector('.modal-title').textContent = 'Create Provider';
    // Show API key field only if needed (handled by change listener)
    const apiKeyGroup = document.getElementById('provider-api-key-group');
    if (apiKeyGroup) apiKeyGroup.style.display = 'none';
    modals.open('provider-modal');
  }

  async function openEditModal(providerId) {
    const provider = providersList.find((p) => p.id === providerId);
    if (!provider) return;

    const modal = document.getElementById('provider-modal');
    if (!modal) return;
    const form = modal.querySelector('form');
    form.reset();

    modal.dataset.mode = 'edit';
    modal.dataset.providerId = providerId;
    modal.querySelector('.modal-title').textContent = 'Edit Provider';

    // Populate fields
    form.display_name.value = provider.display_name;
    form.base_url.value = provider.base_url || '';
    form.auth_type.value = provider.auth_type;
    form.enabled.checked = provider.enabled;
    form.supports_model_discovery.checked = provider.supports_model_discovery;
    // API key field left blank; user must re-enter to change
    const apiKeyGroup = document.getElementById('provider-api-key-group');
    if (apiKeyGroup) apiKeyGroup.style.display = provider.auth_type === 'none' ? 'none' : 'block';

    modals.open('provider-modal');
  }

  function closeModal() {
    const modal = document.getElementById('provider-modal');
    if (modal) {
      modals.close('provider-modal');
      modal.dataset.mode = '';
      modal.dataset.providerId = '';
    }
  }

  async function saveProvider(e) {
    e.preventDefault();
    const form = e.target;
    const modal = document.getElementById('provider-modal');
    const mode = modal.dataset.mode;
    const providerId = modal.dataset.providerId;

    const data = {
      display_name: form.display_name.value.trim(),
      base_url: form.base_url.value.trim(),
      auth_type: form.auth_type.value,
      enabled: form.enabled.checked,
      supports_model_discovery: form.supports_model_discovery.checked,
    };
    if (form.api_key.value) {
      data.api_key = form.api_key.value;
    }

    try {
      if (mode === 'create') {
        await api.providers.create(data);
        state.addToast('Provider created', 'success');
      } else {
        await api.providers.update(providerId, data);
        state.addToast('Provider updated', 'success');
      }
      closeModal();
      loadProviders();
    } catch (error) {
      state.addToast(`Error: ${error.message}`, 'error');
    }
  }

  async function confirmDelete(providerId) {
    const confirmed = confirm('Are you sure you want to delete this provider? This cannot be undone.');
    if (!confirmed) return;

    try {
      await api.providers.delete(providerId);
      state.addToast('Provider deleted', 'success');
      loadProviders();
    } catch (error) {
      state.addToast(`Delete failed: ${error.message}`, 'error');
    }
  }

  async function refreshModels(providerId) {
    try {
      const result = await api.providers.refreshModels(providerId);
      state.addToast(`Models refreshed: ${result.total} found (${result.added} added, ${result.updated} updated)`, 'success');
      // Reload models for this provider
      const models = await api.providers.listModels(providerId);
      state.setProviderModelsCache(providerId, models);
      render();
    } catch (error) {
      state.addToast(`Refresh failed: ${error.message}`, 'error');
    }
  }

  async function toggleModel(modelId, enabled, checkbox) {
    try {
      await api.providerModels.update(modelId, { enabled });
      // Update local cache
      // Find which provider this model belongs to
      for (const pid in state.get('providerModelsCache')) {
        const models = state.getProviderModelsCache(pid);
        const idx = models.findIndex((m) => m.id === modelId);
        if (idx !== -1) {
          models[idx].enabled = enabled;
          state.setProviderModelsCache(pid, models);
          break;
        }
      }
      state.addToast(`Model ${enabled ? 'enabled' : 'disabled'}`, 'success');
      render();
    } catch (error) {
      state.addToast(`Toggle failed: ${error.message}`, 'error');
      // Revert checkbox
      checkbox.checked = !enabled;
    }
  }

  // Expose for modal binding
  function getProviderForEdit() {
    const modal = document.getElementById('provider-modal');
    if (!modal) return null;
    return providersList.find((p) => p.id === modal.dataset.providerId) || null;
  }

  return {
    init,
    loadProviders,
    render,
    openCreateModal,
    openEditModal,
    closeModal,
    saveProvider,
    getProviderForEdit,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = providers;
}
