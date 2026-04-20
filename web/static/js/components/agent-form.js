/**
 * Agent Form Component for Clawdock Dashboard
 * Handles agent creation and editing
 */

const agentForm = (() => {
  let providers = [];
  let models = {};

  function init() {
    loadProviders();
    setupEventListeners();
  }

  async function loadProviders() {
    try {
      providers = await api.providers.list();
      populateProviderSelects();
    } catch (error) {
      console.error('Failed to load providers:', error);
    }
  }

  async function loadModels(providerId) {
    if (!providerId) return;
    if (models[providerId]) return;

    try {
      const providerModels = await api.providers.models(providerId);
      models[providerId] = providerModels;
    } catch (error) {
      console.error('Failed to load models:', error);
    }
  }

  function populateProviderSelects() {
    if (!Array.isArray(providers)) {
      console.warn('populateProviderSelects: providers is not an array', providers);
      return;
    }
    const selects = document.querySelectorAll('.provider-select');
    selects.forEach((select) => {
      select.innerHTML = `
        <option value="">Select Provider...</option>
        ${providers.map((p) => `<option value="${p.id}">${p.display_name}</option>`).join('')}
      `;
    });
  }

  function populateModelSelect(select, providerId) {
    const providerModels = models[providerId] || [];
    select.innerHTML = `
      <option value="">Select Model...</option>
      ${providerModels.map((m) => `<option value="${m.model_key}">${m.display_name}</option>`).join('')}
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
          <p class="form-hint">Provider cannot be changed after creation.</p>
        </div>

        <div class="form-group">
          <label class="form-label">Model</label>
          <input type="text" name="model_id" class="form-input" value="${agent.model_id}">
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
          <p class="form-hint">Optional: JSON object with additional environment variables.</p>
        </div>

        <div class="form-group">
          <label class="form-label">Command Override (JSON)</label>
          <textarea name="command_override_json" class="form-textarea" rows="3" placeholder='["command", "arg1", "arg2"]'>${agent.command_override_json || ''}</textarea>
          <p class="form-hint">Optional: JSON array to override the container command.</p>
        </div>

        <div id="edit-form-errors" class="form-group"></div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" data-action="cancel-edit">Cancel</button>
          <button type="submit" class="btn btn-primary">Save Changes</button>
        </div>
      </form>
    `;
  }

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
            <input type="text" name="image_tag" class="form-input" placeholder="latest" value="latest">
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

        <div class="form-group">
          <label class="form-label">Telegram API Key</label>
          <input type="password" name="telegram_api_key" class="form-input" placeholder="Optional">
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

  function setupEventListeners() {
    // Create form
    document.addEventListener('submit', async (e) => {
      if (e.target.id === 'create-agent-form') {
        e.preventDefault();
        await handleCreate(e.target);
      }
      if (e.target.id === 'edit-agent-form') {
        e.preventDefault();
        await handleEdit(e.target);
      }
    });

    // Provider change
    document.addEventListener('change', async (e) => {
      if (e.target.name === 'provider_id') {
        const providerId = e.target.value;
        const modelSelect = e.target.closest('form').querySelector('.model-select');
        if (providerId) {
          await loadModels(providerId);
          populateModelSelect(modelSelect, providerId);
        } else {
          modelSelect.innerHTML = '<option value="">Select Provider first...</option>';
        }
      }
    });

    // Cancel buttons
    document.addEventListener('click', (e) => {
      if (e.target.matches('[data-action="cancel-create"]')) {
        modals.close('create-agent-modal');
      }
      if (e.target.matches('[data-action="cancel-edit"]')) {
        // Stay on config tab but reset form
        render();
      }
      if (e.target.matches('[data-action="validate-path"]')) {
        const input = e.target.closest('.input-group').querySelector('input');
        validatePath(input.value);
      }
    });
  }

  async function handleCreate(form) {
    const formData = new FormData(form);
    const data = Object.fromEntries(formData.entries());

    // Validate
    const { isValid, errors } = validation.validateAgent(data);
    if (!isValid) {
      displayErrors('create-form-errors', errors);
      return;
    }

    try {
      state.setLoading('action', true);
      await api.agents.create(data);
      toasts.success('Agent created successfully');
      modals.close('create-agent-modal');
      form.reset();
      agentsList.refresh();
    } catch (error) {
      toasts.error(`Failed to create agent: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  async function handleEdit(form) {
    const agentId = form.dataset.agentId;
    const formData = new FormData(form);
    const data = Object.fromEntries(formData.entries());

    // Remove empty optional fields
    Object.keys(data).forEach((key) => {
      if (!data[key]) delete data[key];
    });

    // Validate
    const { isValid, errors } = validation.validateAgent(data);
    if (!isValid) {
      displayErrors('edit-form-errors', errors);
      return;
    }

    try {
      state.setLoading('action', true);
      await api.agents.update(agentId, data);
      toasts.success('Agent updated successfully');
      agentsList.refresh();
      // Switch to overview tab
      document.querySelector('[data-tab="overview"]')?.click();
    } catch (error) {
      toasts.error(`Failed to update agent: ${error.message}`);
    } finally {
      state.setLoading('action', false);
    }
  }

  async function validatePath(path) {
    if (!path) {
      toasts.warning('Please enter a path to validate');
      return;
    }
    try {
      await api.validate.path(path);
      toasts.success('Path is valid');
    } catch (error) {
      toasts.error('Path validation failed: ' + error.message);
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
        <strong>Please fix the following errors:</strong>
        <ul style="margin-top: 0.5rem; padding-left: 1.5rem;">
          ${Object.entries(errors).map(([field, message]) => `
            <li>${format.capitalize(field.replace('_', ' '))}: ${message}</li>
          `).join('')}
        </ul>
      </div>
    `;
  }

  return {
    init,
    renderCreateForm,
    renderEditForm,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = agentForm;
}
