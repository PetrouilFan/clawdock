/**
 * Agent Form Component
 * Handles creating and editing agents
 */

const agentForm = (() => {
  function init() {
    loadProviders();
    setupEventListeners();
  }

  async function loadProviders() {
    try {
      const list = await api.providers.list();
      state.set('providers', list);
      populateProviderSelects();
    } catch (error) {
      console.error('Failed to load providers:', error);
    }
  }

  function populateProviderSelects() {
    const providers = state.get('providers') || [];
    document.querySelectorAll('.provider-select').forEach((select) => {
      const current = select.value;
      select.innerHTML = `<option value="">Select Provider...</option>` +
        providers.map((p) => `<option value="${p.id}" ${p.id === current ? 'selected' : ''}>${p.display_name}</option>`).join('');
    });
  }

  function populateModelSelect(select, providerId) {
    const models = state.getProviderModelsCache(providerId) || [];
    select.innerHTML = `<option value="">Select Model...</option>` +
      models.map((m) => `<option value="${m.model_key}">${m.display_name}</option>`).join('');
  }

  function setupEventListeners() {
    // Provider change: load models and toggle API key field
    document.addEventListener('change', (e) => {
      if (e.target.matches('select.provider-select')) {
        const form = e.target.closest('form');
        const providerId = e.target.value;
        const modelSelect = form.querySelector('.model-select');
        const apiKeyGroup = form.querySelector('.provider-api-key-group');

        if (apiKeyGroup) {
          const providers = state.get('providers') || [];
          const provider = providers.find((p) => p.id === providerId);
          if (provider && provider.auth_type === 'none') {
            apiKeyGroup.style.display = 'none';
          } else {
            apiKeyGroup.style.display = 'block';
          }
        }

        if (providerId) {
          const cached = state.getProviderModelsCache(providerId);
          if (cached.length === 0) {
            api.providers.listModels(providerId).then((models) => {
              state.setProviderModelsCache(providerId, models);
              populateModelSelect(modelSelect, providerId);
            }).catch(console.error);
          } else {
            populateModelSelect(modelSelect, providerId);
          }
        } else {
          modelSelect.innerHTML = '<option value="">Select Provider first...</option>';
          if (apiKeyGroup) apiKeyGroup.style.display = 'none';
        }
      }
    });

    // Cancel buttons & validate path
    document.addEventListener('click', (e) => {
      if (e.target.matches('[data-action="cancel-create"]')) {
        modals.close('create-agent-modal');
      }
      if (e.target.matches('[data-action="cancel-edit"]')) {
        modals.close('agent-detail-modal');
      }
      if (e.target.matches('[data-action="validate-path"]')) {
        const input = e.target.closest('.input-group').querySelector('input');
        validatePath(input.value);
      }
    });
  }

  async function handleCreate(form) {
    const data = Object.fromEntries(new FormData(form));
    const { isValid, errors } = validation.validateAgent(data);
    if (!isValid) {
      displayErrors('create-form-errors', errors);
      return;
    }
    try {
      state.setLoading('action', true);
      await api.agents.create(data);
      state.addToast('Agent created', 'success');
      modals.close('create-agent-modal');
      form.reset();
      agentsList.refresh();
    } catch (error) {
      state.addToast(`Create failed: ${error.message}`, 'error');
    } finally {
      state.setLoading('action', false);
    }
  }

  async function handleEdit(form) {
    const agentId = form.dataset.agentId;
    const data = Object.fromEntries(new FormData(form));
    Object.keys(data).forEach((k) => !data[k] && delete data[k]);

    const { isValid, errors } = validation.validateAgent(data);
    if (!isValid) {
      displayErrors('edit-form-errors', errors);
      return;
    }
    try {
      state.setLoading('action', true);
      await api.agents.update(agentId, data);
      state.addToast('Agent updated', 'success');
      agentsList.refresh();
      // Return to overview
      document.querySelector('[data-tab="overview"]')?.click();
    } catch (error) {
      state.addToast(`Update failed: ${error.message}`, 'error');
    } finally {
      state.setLoading('action', false);
    }
  }

  async function validatePath(path) {
    if (!path) {
      state.addToast('Enter a path to validate', 'warning');
      return;
    }
    try {
      await api.validate.path(path);
      state.addToast('Path is valid', 'success');
    } catch (error) {
      state.addToast(`Path invalid: ${error.message}`, 'error');
    }
  }

  function displayErrors(containerId, errors) {
    const container = document.getElementById(containerId);
    if (!container) return;
    if (Object.keys(errors).length === 0) {
      container.innerHTML = '';
      return;
    }
    container.innerHTML = `
      <div class="form-error">
        <strong>Please fix:</strong>
        <ul style="margin-top:0.5rem;padding-left:1.5rem;">
          ${Object.entries(errors).map(([field, msg]) => `<li>${field.replace('_', ' ')}: ${msg}</li>`).join('')}
        </ul>
      </div>
    `;
  }

  // Public API
  function renderCreateForm() {
    return `
      <form id="create-agent-form">
        <div class="form-group">
          <label class="form-label">Name <span class="required">*</span></label>
          <input type="text" name="name" class="form-input" placeholder="My Agent" required>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label class="form-label">Image Tag</label>
            <input type="text" name="image_tag" class="form-input" value="latest">
          </div>
          <div class="form-group">
            <label class="form-label">Restart Policy</label>
            <select name="restart_policy" class="form-select">
              <option value="always">Always</option>
              <option value="unless-stopped" selected>Unless Stopped</option>
              <option value="no">No</option>
            </select>
          </div>
        </div>
        <div class="form-group">
          <label class="form-label">Provider <span class="required">*</span></label>
          <select name="provider_id" class="form-select provider-select" required>
            <option value="">Select Provider...</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">Model <span class="required">*</span></label>
          <select name="model_id" class="form-select model-select" required>
            <option value="">Select Provider first...</option>
          </select>
        </div>
        <div class="form-group provider-api-key-group" style="display:none;">
          <label class="form-label">Provider API Key</label>
          <input type="password" name="telegram_api_key" class="form-input" placeholder="Optional override">
          <p class="form-hint">If left blank, the provider's stored key will be used.</p>
        </div>
        <div class="form-group">
          <label class="form-label">Workspace Host Path</label>
          <div class="input-group">
            <input type="text" name="workspace_host_path" class="form-input" placeholder="/data/workspaces/my-agent">
            <button type="button" class="btn btn-secondary" data-action="validate-path">Validate</button>
          </div>
        </div>
        <div class="form-group">
          <label class="form-label">Workspace Container Path</label>
          <input type="text" name="workspace_container_path" class="form-input" value="/workspace">
        </div>
        <div class="form-group">
          <label class="form-label">Extra Environment Variables (JSON)</label>
          <textarea name="extra_env_json" class="form-textarea" rows="3" placeholder='{"KEY": "value"}'></textarea>
        </div>
        <div id="create-form-errors" class="form-group"></div>
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" data-action="cancel-create">Cancel</button>
          <button type="submit" class="btn btn-primary">Create Agent</button>
        </div>
      </form>
    `;
  }

  function renderEditForm(agent) {

    return `
      <form id="edit-agent-form" data-agent-id="${agent.id}">
        <div class="form-group">
          <label class="form-label">Name</label>
          <input type="text" name="name" class="form-input" value="${agent.name}" required>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label class="form-label">Image Tag</label>
            <input type="text" name="image_tag" class="form-input" value="${agent.image_tag}">
          </div>
          <div class="form-group">
            <label class="form-label">Restart Policy</label>
            <select name="restart_policy" class="form-select">
              <option value="always" ${agent.restart_policy === 'always' ? 'selected' : ''}>Always</option>
              <option value="unless-stopped" ${agent.restart_policy === 'unless-stopped' ? 'selected' : ''}>Unless Stopped</option>
              <option value="no" ${agent.restart_policy === 'no' ? 'selected' : ''}>No</option>
            </select>
          </div>
        </div>
        <div class="form-group">
          <label class="form-label">Provider</label>
          <input type="text" class="form-input" value="${format.providerDisplayName(agent.provider_id)}" disabled>
        </div>
        <div class="form-group">
          <label class="form-label">Model</label>
          <select name="model_id" class="form-select model-select" required>
            <option value="">Select Model...</option>
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">Telegram API Key</label>
          <input type="password" name="telegram_api_key" class="form-input" placeholder="Leave blank to keep unchanged">
        </div>
        <div class="form-group">
          <label class="form-label">Workspace Host Path</label>
          <div class="input-group">
            <input type="text" name="workspace_host_path" class="form-input" value="${agent.workspace_host_path || ''}">
            <button type="button" class="btn btn-secondary" data-action="validate-path">Validate</button>
          </div>
        </div>
        <div class="form-group">
          <label class="form-label">Workspace Container Path</label>
          <input type="text" name="workspace_container_path" class="form-input" value="${agent.workspace_container_path || '/workspace'}">
        </div>
        <div class="form-group">
          <label class="form-label">Extra Environment Variables (JSON)</label>
          <textarea name="extra_env_json" class="form-textarea" rows="3" placeholder='{"KEY": "value"}'>${agent.extra_env_json || ''}</textarea>
        </div>
        <div id="edit-form-errors" class="form-group"></div>
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" data-action="cancel-edit">Cancel</button>
          <button type="submit" class="btn btn-primary">Save Changes</button>
        </div>
      </form>
    `;
  }

  function afterRender(agent) {
    const modal = document.getElementById('agent-detail-modal');
    if (!modal) return;
    const select = modal.querySelector('.model-select');
    if (!select) return;
    const providerId = agent.provider_id;
    const cached = state.getProviderModelsCache(providerId);
    if (cached.length > 0) {
      populateModelSelect(select, providerId);
      select.value = agent.model_id;
    } else {
      api.providers.listModels(providerId).then((models) => {
        state.setProviderModelsCache(providerId, models);
        populateModelSelect(select, providerId);
        select.value = agent.model_id;
      }).catch((err) => {
        console.error('Failed to load models for edit form:', err);
      });
    }
  }

  return {
    init,
    renderCreateForm,
    renderEditForm,
    afterRender,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = agentForm;
}
