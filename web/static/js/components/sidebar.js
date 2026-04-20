/**
 * Sidebar Component for Clawdock Dashboard
 */

const sidebar = (() => {
  function init() {
    render();
    setupEventListeners();
  }

  function render() {
    const sidebarEl = document.getElementById('sidebar');
    if (!sidebarEl) return;

    const currentView = state.get('currentView');

    const navItems = [
      { id: 'agents', label: 'Agents', icon: '🤖' },
      { id: 'providers', label: 'Providers', icon: '🔌' },
      { id: 'audit', label: 'Audit Log', icon: '📋' },
      { id: 'system', label: 'System', icon: '⚙️' }
    ];

    const navHTML = navItems.map(item => `
      <a href="#${item.id}" class="nav-link ${currentView === item.id ? 'active' : ''}" data-view="${item.id}">
        <span class="nav-icon">${item.icon}</span>
        <span>${item.label}</span>
      </a>
    `).join('');

    const navContainer = sidebarEl.querySelector('.sidebar-nav');
    if (navContainer) {
      navContainer.innerHTML = `
        <div class="nav-section">
          ${navHTML}
        </div>
      `;
    }
  }

  function setupEventListeners() {
    const sidebarEl = document.getElementById('sidebar');
    if (!sidebarEl) return;

    sidebarEl.addEventListener('click', (e) => {
      const link = e.target.closest('.nav-link');
      if (link) {
        e.preventDefault();
        const view = link.dataset.view;
        if (view) {
          state.setView(view);
        }
      }
    });

    // Listen for view changes
    state.on('currentView', () => {
      render();
    });
  }

  function toggle() {
    const sidebarEl = document.getElementById('sidebar');
    if (sidebarEl) {
      sidebarEl.classList.toggle('open');
      state.set('sidebarOpen', sidebarEl.classList.contains('open'));
    }
  }

  return {
    init,
    toggle
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = sidebar;
}
