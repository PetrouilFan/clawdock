/**
 * Toast Notifications for Clawdock Dashboard
 */

const toasts = (() => {
  let container = null;

  function init() {
    container = document.getElementById('toast-container');
    if (!container) {
      container = dom.createElement('div', {
        id: 'toast-container',
        className: 'toast-container'
      });
      document.body.appendChild(container);
    }

    // Listen for state changes
    state.on('toasts', render);
  }

  function render(toasts) {
    if (!container) return;

    // Only render new toasts that aren't already in the DOM
    const existingIds = new Set(
      Array.from(container.children).map(el => parseInt(el.dataset.id))
    );

    toasts.forEach(toast => {
      if (!existingIds.has(toast.id)) {
        const el = createToastElement(toast);
        container.appendChild(el);
      }
    });

    // Remove toasts that are no longer in state
    Array.from(container.children).forEach(el => {
      const id = parseInt(el.dataset.id);
      if (!toasts.find(t => t.id === id)) {
        el.remove();
      }
    });
  }

  function createToastElement(toast) {
    const el = dom.createElement('div', {
      className: `toast ${toast.type}`,
      attributes: { 'data-id': toast.id }
    });

    const icon = getIconForType(toast.type);

    el.innerHTML = `
      <span class="toast-icon">${icon}</span>
      <span class="toast-message">${dom.escapeHtml(toast.message)}</span>
      <button class="toast-close" aria-label="Close">&times;</button>
    `;

    el.querySelector('.toast-close').addEventListener('click', () => {
      state.removeToast(toast.id);
    });

    return el;
  }

  function getIconForType(type) {
    const icons = {
      success: '✓',
      error: '✕',
      warning: '⚠',
      info: 'ℹ'
    };
    return icons[type] || icons.info;
  }

  function success(message) {
    state.addToast(message, 'success');
  }

  function error(message) {
    state.addToast(message, 'error');
  }

  function warning(message) {
    state.addToast(message, 'warning');
  }

  function info(message) {
    state.addToast(message, 'info');
  }

  return {
    init,
    success,
    error,
    warning,
    info
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = toasts;
}
