/**
 * Settings Component
 * Global configuration for the model router
 */

const settingsComponent = (() => {
  let initialized = false;

  function init() {
    if (initialized) return;
    loadSettings();
    setupEventListeners();
    initialized = true;
  }

  async function loadSettings() {
    try {
      const [defaultModel, chatEnabled] = await Promise.all([
        api.settings.getDefaultModel(),
        api.settings.getChatProxyEnabled(),
      ]);
      state.setSettings({
        default_model: defaultModel.default_model || '',
        chat_proxy_enabled: chatEnabled.enabled,
      });
      render();
    } catch (error) {
      console.error('Failed to load settings:', error);
    } finally {
      state.setLoading('settings', false);
    }
  }

  function setupEventListeners() {
    document.addEventListener('click', (e) => {
      if (e.target.matches('#btn-settings')) {
        state.openModal('settingsModal');
      }
    });
  }

  function render() {
    const container = document.getElementById('settings-content');
    if (!container) return;

    const settings = state.get('settings');
    const allProviders = state.get('providers');
    const allCustomModels = state.get('customModels');

    // Build model options: from provider models + custom models
    let modelOptions = '';
    allProviders.forEach((p) => {
      const models = state.getProviderModelsCache(p.id) || [];
      models.forEach((m) => {
        if (m.enabled) {
          modelOptions += `<option value="${m.model_key}" ${m.model_key === settings.default_model ? 'selected' : ''}>${m.display_name} (${p.display_name})</option>`;
        }
      });
    });
    allCustomModels.forEach((c) => {
      if (c.enabled) {
        modelOptions += `<option value="${c.id}" ${c.id === settings.default_model ? 'selected' : ''}>${c.id} (Custom → ${c.target_provider_id}/${c.target_model_key})</option>`;
      }
    });

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <div class="card-title">Settings</div>
        </div>
        <div class="card-body">
          <div class="form-group">
            <label for="setting-default-model">Default Model</label>
            <p class="text-muted" style="font-size:0.875rem; margin-bottom:0.5rem;">Used when no model is specified in chat requests.</p>
            <select id="setting-default-model" class="form-select">
              <option value="">(none)</option>
              ${modelOptions}
            </select>
          </div>

          <div class="form-group" style="margin-top:1.5rem;">
            <label class="checkbox-label">
              <input type="checkbox" id="setting-chat-proxy-enabled" ${settings.chat_proxy_enabled ? 'checked' : ''}>
              <span>Enable Chat Proxy</span>
            </label>
            <p class="text-muted" style="font-size:0.875rem;">When disabled, requests to /v1/chat/completions will be rejected.</p>
          </div>

          <div style="margin-top:1.5rem;">
            <button class="btn btn-primary" id="btn-save-settings">Save Settings</button>
          </div>
        </div>
      </div>
    `;
  }

  function setupEventListeners() {
    // Already bound in init
  }

  async function saveSettings() {
    try {
      const defaultModel = document.getElementById('setting-default-model').value;
      const chatEnabled = document.getElementById('setting-chat-proxy-enabled').checked;

      await Promise.all([
        api.settings.setDefaultModel(defaultModel),
        api.settings.setChatProxyEnabled(chatEnabled),
      ]);
      state.addToast('Settings saved', 'success');
      state.closeModal('settingsModal');
      // Update local state
      state.setSettings({ default_model: defaultModel, chat_proxy_enabled: chatEnabled });
    } catch (error) {
      state.addToast(`Save failed: ${error.message}`, 'error');
    }
  }

  // Bind save button when modal opens
  function bindEvents() {
    const saveBtn = document.getElementById('btn-save-settings');
    if (saveBtn) {
      saveBtn.onclick = saveSettings;
    }
  }

  // Expose bindEvents to be called after modal open
  return {
    init,
    render,
    bindEvents,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = settingsComponent;
}
