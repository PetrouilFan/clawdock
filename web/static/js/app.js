/**
 * Clawdock Dashboard - Main Application
 * Initializes all components and starts the application
 */

const app = (() => {
  function init() {
    // Check if all dependencies are loaded
    if (!checkDependencies()) {
      console.error('Required dependencies not loaded');
      return;
    }

    console.log('🦀 Clawdock Dashboard initializing...');

    // Initialize all components
    sidebar.init();
    agentsList.init();
    agentDetail.init();
    agentForm.init();
    toasts.init();
    modals.init();
    terminal.init();
    backups.init();
    providers.init();
    auditLog.init();

    // Setup view routing
    setupViewRouting();

    // Load initial data
    loadInitialData();

    console.log('🦀 Clawdock Dashboard initialized');
  }

  function checkDependencies() {
    const required = ['api', 'state', 'dom', 'format', 'validation'];
    const missing = required.filter((dep) => typeof window[dep] === 'undefined');

    if (missing.length > 0) {
      console.error('Missing dependencies:', missing);
      // Show visible error on page
      const content = document.querySelector('.content');
      if (content) {
        content.innerHTML = `
          <div class="empty-state" style="color: var(--accent-danger);">
            <div class="empty-state-icon">❌</div>
            <div class="empty-state-title">Failed to load JavaScript modules</div>
            <div class="empty-state-text">
              Missing: ${missing.join(', ')}<br>
              Please check your internet connection or try refreshing.
            </div>
            <button class="btn btn-primary" onclick="location.reload()">Reload Page</button>
          </div>
        `;
      }
      return false;
    }

    return true;
  }

  function setupViewRouting() {
    // Handle hash changes for view switching
    window.addEventListener('hashchange', handleRoute);

    // Initial route
    handleRoute();

    // Listen for state changes
    state.on('currentView', (view) => {
      showView(view);
      updateSidebarActive(view);
    });
  }

  function handleRoute() {
    const hash = window.location.hash.slice(1) || 'agents';
    state.setView(hash);
  }

  function showView(viewName) {
    // Hide all views
    document.querySelectorAll('.view').forEach((view) => {
      view.classList.add('hidden');
    });

    // Show requested view
    const viewElement = document.getElementById(`${viewName}-view`);
    if (viewElement) {
      viewElement.classList.remove('hidden');
    }

    // Update page title
    updatePageTitle(viewName);
  }

  function updatePageTitle(viewName) {
    const titles = {
      agents: 'Agents',
      providers: 'Providers',
      audit: 'Audit Log',
      system: 'System Status',
    };
    const title = titles[viewName] || viewName;
    document.title = `${title} - Clawdock`;

    // Update header title
    const headerTitle = document.getElementById('header-title');
    if (headerTitle) {
      headerTitle.textContent = title;
    }
  }

  function updateSidebarActive(viewName) {
    document.querySelectorAll('.nav-link').forEach((link) => {
      link.classList.remove('active');
      if (link.dataset.view === viewName) {
        link.classList.add('active');
      }
    });
  }

  async function loadInitialData() {
    // Load system status
    await loadSystemStatus();

    // Setup periodic system status refresh
    setInterval(loadSystemStatus, 30000);
  }

  async function loadSystemStatus() {
    try {
      const [health, ready, version] = await Promise.all([
        api.health.check().catch(() => null),
        api.health.ready().catch(() => null),
        api.health.version().catch(() => 'unknown'),
      ]);

      const statusBadge = document.getElementById('health-status');
      if (statusBadge) {
        const isHealthy = health === 'ok' && ready !== null;
        statusBadge.className = `status-badge ${isHealthy ? 'online' : 'offline'}`;
        statusBadge.textContent = isHealthy ? 'Healthy' : 'Unhealthy';
      }

      state.set('systemStatus', {
        health,
        ready,
        version,
        timestamp: new Date().toISOString(),
      });
    } catch (error) {
      console.error('Failed to load system status:', error);
    }
  }

  return {
    init,
  };
})();

// Initialize app when DOM is ready
dom.onReady(() => {
  app.init();
});

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
  module.exports = app;
}
