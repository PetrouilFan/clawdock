/**
 * Modal Components for Clawdock Dashboard
 */

const modals = (() => {
  let confirmCallback = null;
  let confirmData = null;

  function init() {
    setupEventListeners();
    setupConfirmModal();
  }

  function setupEventListeners() {
    // Close modals on escape key
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        closeAll();
      }
    });

    // Close on overlay click
    document.querySelectorAll('.modal-overlay').forEach((overlay) => {
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
          const modal = overlay.querySelector('.modal');
          if (modal && !modal.dataset.preventClose) {
            close(overlay);
          }
        }
      });
    });
  }

  function setupConfirmModal() {
    const confirmBtn = document.getElementById('confirm-btn');
    const cancelBtn = document.getElementById('cancel-btn');

    if (confirmBtn) {
      confirmBtn.addEventListener('click', () => {
        if (confirmCallback) {
          confirmCallback(confirmData);
        }
        close(document.getElementById('confirm-modal'));
      });
    }

    if (cancelBtn) {
      cancelBtn.addEventListener('click', () => {
        close(document.getElementById('confirm-modal'));
      });
    }
  }

  function open(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
      modal.classList.add('active');
      document.body.style.overflow = 'hidden';
      state.openModal(modalId.replace('-modal', '').replace(/-/g, '_'));
    }
  }

  function close(modal) {
    if (typeof modal === 'string') {
      modal = document.getElementById(modal);
    }
    if (modal) {
      modal.classList.remove('active');
      document.body.style.overflow = '';
      state.closeModal(modal.id.replace('-modal', '').replace(/-/g, '_'));
    }
  }

  function closeAll() {
    document.querySelectorAll('.modal-overlay.active').forEach((modal) => {
      close(modal);
    });
  }

  function confirm(options) {
    const { title, message, confirmText = 'Confirm', cancelText = 'Cancel', type = 'danger', onConfirm, data } = options;

    const modal = document.getElementById('confirm-modal');
    const titleEl = document.getElementById('confirm-title');
    const messageEl = document.getElementById('confirm-message');
    const confirmBtn = document.getElementById('confirm-btn');
    const cancelBtn = document.getElementById('cancel-btn');

    if (titleEl) titleEl.textContent = title;
    if (messageEl) messageEl.textContent = message;
    if (confirmBtn) {
      confirmBtn.textContent = confirmText;
      confirmBtn.className = `btn btn-${type}`;
    }
    if (cancelBtn) cancelBtn.textContent = cancelText;

    confirmCallback = onConfirm;
    confirmData = data;

    open('confirm-modal');
  }

  function openAgentCreate() {
    open('create-agent-modal');
    const form = document.getElementById('create-agent-form');
    if (form) form.reset();
  }

  function openAgentDetail(agentId) {
    state.selectAgent(agentId);
    open('agent-detail-modal');
  }

  function openTerminal(agentId) {
    state.selectAgent(agentId);
    open('terminal-modal');
    terminal.connect(agentId);
  }

  function openBackup(agentId) {
    state.selectAgent(agentId);
    open('backup-modal');
  }

  return {
    init,
    open,
    close,
    closeAll,
    confirm,
    openAgentCreate,
    openAgentDetail,
    openTerminal,
    openBackup,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = modals;
}
