/**
 * Providers Component for Clawdock Dashboard
 */

const providers = (() => {
  let providers = [];
  let providerModels = {};

  function init() {
    loadProviders();
  }

  async function loadProviders() {
    try {
      providers = await api.providers.list();
      state.set('providers', providers);
      render();
    } catch (error) {
      console.error('Failed to load providers:', error);
    }
  }

  async function loadModels(providerId) {
    if (providerModels[providerId]) return;

    try {
      const models = await api.providers.models(providerId);
      providerModels[providerId] = models;
    } catch (error) {
      console.error('Failed to load models:', error);
    }
  }

  function render() {
    const container = document.getElementById('providers-content');
    if (!container) return;

    if (providers.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <div class="empty-state-icon">🔌</div>
          <div class="empty-state-title">No providers available</div>
          <div class="empty-state-text">Providers will be seeded when the application starts.</div>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card">
        <div class="card-header">
          <div class="card-title">AI Providers</div>
          <span class="text-muted">${providers.length} provider${providers.length !== 1 ? 's' : ''}</span>
        </div>
        <div class="card-body">
          <div class="providers-grid" style="display: grid; gap: 1.5rem;">
            ${providers.map(provider => renderProviderCard(provider)).join('')}
          </div>
        </div>
      </div>
    `;

    // Load models for each provider
    providers.forEach((p) => loadModels(p.id));
  }

  function renderProviderCard(provider) {
    const models = providerModels[provider.id] || [];
    const enabledModels = models.filter((m) => m.enabled);

    return `
      <div class="provider-card" data-provider-id="${provider.id}">
        <div class="provider-header">
          <div class="provider-info">
            <div class="provider-icon">${getProviderIcon(provider.id)}</div>
            <div>
              <div class="provider-name">${provider.display_name}</div>
              <div class="provider-meta">
                ${provider.auth_type !== 'none' ? '🔐 Authentication required' : 'No authentication'}
                ${provider.base_url ? `• ${provider.base_url}` : ''}
              </div>
            </div>
          </div>
          <div class="provider-status">
            ${provider.enabled
              ? '<span class="badge badge-success">Enabled</span>'
              : '<span class="badge badge-neutral">Disabled</span>'
            }
          </div>
        </div>

        <div class="provider-models">
          <div style="padding: 1rem 1.5rem; border-bottom: 1px solid var(--border-primary);">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="text-muted" style="font-size: 0.875rem;">${models.length} model${models.length !== 1 ? 's' : ''} (${enabledModels.length} enabled)</span>
              <button class="btn btn-sm btn-ghost" data-action="toggle-models" data-provider-id="${provider.id}">
                Show/Hide
              </button>
            </div>
          </div>
          <div class="models-list">
            ${models.length === 0
              ? '<div style="padding: 1rem 1.5rem; color: var(--text-muted); font-size: 0.875rem;">No models available</div>'
              : models.map((model) => `
                <div class="model-row">
                  <div>
                    <div class="model-name">${model.display_name}</div>
                    <div class="model-key">${model.model_key}</div>
                  </div>
                  <div>
                    ${model.enabled
                      ? '<span class="badge badge-success">Enabled</span>'
                      : '<span class="badge badge-neutral">Disabled</span>'
                    }
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
      ollama: '🅞',
      openrouter: '⛐',
      custom: '⚙️',
    };
    return icons[providerId] || '🤖';
  }

  return {
    init,
    render,
    loadProviders,
    loadModels,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = providers;
}
