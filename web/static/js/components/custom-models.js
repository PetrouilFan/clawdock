/**
 * Custom Models Component
 * Manages alias mappings to provider models
 */

const customModels = (() => {
  let customModelsList = [];
  let allProviders = [];

  function init() {
    loadCustomModels();
    loadProvidersForSelects();
    setupEventListeners();
  }

  async function loadCustomModels() {
    try {
      customModelsList = await api.customModels.list();
      state.setCustomModels(customModelsList);
      render();
    } catch (error) {
      console.error('Failed to load custom models:', error);
      state.addToast('Failed to load custom models', 'error');
    } finally {
      state.setLoading('customModels', false);
    }
  }

  async function loadProvidersForSelects() {
    try {
      allProviders = await api.providers.list();
      // Populate provider selects in modals when opened
    } catch (error) {
      console.error('Failed to load providers for selects:', error);
    }
  }

  function setupEventListeners() {
    document.addEventListener('click', (e) => {
      if (e.target.matches('#btn-create-custom-model')) {
        openCreateModal();
      }
      if (e.target.matches('.custom-model-edit-btn')) {
        const alias = e.target.closest('[data-alias]').dataset.alias;
        openEditModal(alias);
      }
      if (e.target.matches('.custom-model-delete-btn')) {
        const alias = e.target.closest('[data-alias]').dataset.alias;
        confirmDelete(alias);
      }
    });
  }

  function render() {
    const container = document.getElementById('custom-models-content');
    if (!container) return;

    if (customModelsList.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🔗</div>
          <div class="empty-state-title">No custom models</div>
          <div class="empty-state-text">Create aliases to easily reference models.</div>
          <button class="btn btn-primary" id="btn-create-custom-model">Create Alias</button>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <div class="card-title">Custom Model Aliases</div>
          <button class="btn btn-sm btn-primary" id="btn-create-custom-model">+ Create Alias</button>
        </div>
        <div class="card-body">
          <table class="table">
            <thead>
              <tr>
                <th>Alias</th>
                <th>Target Provider</th>
                <th>Target Model</th>
                <th>Enabled</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              ${customModelsList.map((cm) => `
                <tr data-alias="${cm.id}">
                  <td><code>${cm.id}</code></td>
                  <td>${cm.target_provider_id}</td>
                  <td>${cm.target_model_key}</td>
                  <td>
                    <label class="toggle-switch-sm">
                      <input type="checkbox" ${cm.enabled ? 'checked' : ''} disabled>
                      <span class="slider"></span>
                    </label>
                  </td>
                  <td>
                    <button class="btn btn-sm btn-ghost custom-model-edit-btn" data-alias="${cm.id}">✏️</button>
                    <button class="btn btn-sm btn-ghost custom-model-delete-btn" data-alias="${cm.id}" style="color:var(--danger)">🗑️</button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    `;
  }

  function openCreateModal() {
    const modal = document.getElementById('custom-model-modal');
    if (!modal) return;
    modal.querySelector('form').reset();
    modal.dataset.mode = 'create';
    modal.dataset.alias = '';
    modal.querySelector('.modal-title').textContent = 'Create Custom Model';
    // Populate provider select
    populateProviderSelect();
    state.openModal('customModelModal');
  }

  async function openEditModal(alias) {
    const cm = customModelsList.find((c) => c.id === alias);
    if (!cm) return;

    const modal = document.getElementById('custom-model-modal');
    if (!modal) return;
    modal.querySelector('form').reset();

    modal.dataset.mode = 'edit';
    modal.dataset.alias = alias;
    modal.querySelector('.modal-title').textContent = 'Edit Custom Model';

    const form = modal.querySelector('form');
    form.alias.value = alias; // readonly maybe
    form.target_provider_id.value = cm.target_provider_id;
    form.target_model_key.value = cm.target_model_key;
    form.enabled.checked = cm.enabled;

    await populateProviderSelect(form.target_provider_id.value);
    // After provider selected, load its models for target_model_key select
    // For simplicity, use free-text input for model key, but could be select
    state.openModal('customModelModal');
  }

  function closeModal() {
    const modal = document.getElementById('custom-model-modal');
    if (modal) {
      state.closeModal('customModelModal');
      modal.dataset.mode = '';
      modal.dataset.alias = '';
    }
  }

  async function populateProviderSelect(selected = '') {
    const select = document.getElementById('custom-target-provider');
    if (!select) return;
    select.innerHTML = '<option value="">Select provider...</option>' +
      allProviders.map((p) => `<option value="${p.id}" ${p.id === selected ? 'selected' : ''}>${p.display_name}</option>`).join('');
  }

  // Model key input: free-text for now, but could be a select populated when provider changes
  // We'll add a change listener on provider select to fetch and populate model select

  function setupModelKeySelect(providerId, selectedKey = '') {
    // If we have cached models for provider, populate a select; otherwise free text
    const models = state.getProviderModelsCache(providerId);
    const input = document.getElementById('custom-target-model-key');
    if (!input) return;

    if (models && models.length > 0) {
      // Replace input with select
      const select = document.createElement('select');
      select.id = 'custom-target-model-key';
      select.name = 'target_model_key';
      select.required = true;
      select.innerHTML = '<option value="">Select model...</option>' +
        models.map((m) => `<option value="${m.model_key}" ${m.model_key === selectedKey ? 'selected' : ''}>${m.display_name}</option>`).join('');
      input.replaceWith(select);
    } else {
      // Keep text input, suggest to fetch models first
    }
  }

  async function saveCustomModel(e) {
    e.preventDefault();
    const form = e.target;
    const modal = document.getElementById('custom-model-modal');
    const mode = modal.dataset.mode;
    const alias = modal.dataset.alias || form.alias.value.trim();

    const data = {
      target_provider_id: form.target_provider_id.value,
      target_model_key: form.target_model_key.value.trim(),
      enabled: form.enabled.checked,
    };

    try {
      if (mode === 'create') {
        await api.customModels.create(alias, data);
        state.addToast('Custom model created', 'success');
      } else {
        await api.customModels.update(alias, data);
        state.addToast('Custom model updated', 'success');
      }
      closeModal();
      loadCustomModels();
    } catch (error) {
      state.addToast(`Error: ${error.message}`, 'error');
    }
  }

  function confirmDelete(alias) {
    if (!confirm(`Delete alias "${alias}"?`)) return;
    (async () => {
      try {
        await api.customModels.delete(alias);
        state.addToast('Alias deleted', 'success');
        loadCustomModels();
      } catch (error) {
        state.addToast(`Delete failed: ${error.message}`, 'error');
      }
    })();
  }

  // Event delegation for model key select population
  function handleProviderChange(e) {
    const providerId = e.target.value;
    const selectedKey = '';
    setupModelKeySelect(providerId, selectedKey);
  }

  return {
    init,
    loadCustomModels,
    loadProvidersForSelects,
    openCreateModal,
    openEditModal,
    closeModal,
    saveCustomModel,
    handleProviderChange,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = customModels;
}
