/**
 * Terminal Component for Clawdock Dashboard
 * Uses xterm.js for full terminal emulation
 */

const terminal = (() => {
  let term = null;
  let socket = null;
  let currentAgentId = null;

  function init() {
    // Nothing to do here until terminal is opened
  }

  async function connect(agentId) {
    if (!agentId) {
      agentId = currentAgentId;
    }
    currentAgentId = agentId;

    if (socket && socket.readyState === WebSocket.OPEN) {
      disconnect();
    }

    const container = document.getElementById('terminal-container');
    if (!container) return;

    // Clear container
    container.innerHTML = '';

    // Initialize xterm.js
    if (typeof Terminal === 'undefined') {
      container.innerHTML = `
        <div style="padding: 2rem; text-align: center; color: var(--text-muted);">
          <p>Loading terminal...</p>
          <p class="text-muted" style="font-size: 0.85rem; margin-top: 1rem;">
            If this persists, check that xterm.js is loaded
          </p>
        </div>
      `;
      return;
    }

    term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'JetBrains Mono, Fira Code, Consolas, monospace',
      theme: {
        background: '#000000',
        foreground: '#00ff00',
        cursor: '#00ff00',
        selectionBackground: 'rgba(0, 255, 0, 0.3)',
        black: '#000000',
        red: '#ff0000',
        green: '#00ff00',
        yellow: '#ffff00',
        blue: '#0000ff',
        magenta: '#ff00ff',
        cyan: '#00ffff',
        white: '#ffffff',
      },
      cols: 80,
      rows: 24,
    });

    term.open(container);

    // Handle terminal input
    term.onData((data) => {
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(data);
      }
    });

    // Connect WebSocket
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${location.host}/api/agents/${agentId}/terminal`;

    updateStatus('Connecting...');

    try {
      socket = new WebSocket(wsUrl);

      socket.onopen = () => {
        updateStatus('Connected');
        term.writeln('');
        term.writeln('\x1b[32mConnected to agent terminal\x1b[0m');
        term.writeln('');
      };

      socket.onmessage = (event) => {
        term.write(event.data);
      };

      socket.onclose = () => {
        updateStatus('Disconnected');
        term.writeln('');
        term.writeln('\x1b[31mConnection closed\x1b[0m');
      };

      socket.onerror = (error) => {
        updateStatus('Error');
        term.writeln('');
        term.writeln('\x1b[31mConnection error\x1b[0m');
        console.error('WebSocket error:', error);
      };
    } catch (error) {
      updateStatus('Failed');
      term.writeln('');
      term.writeln(`\x1b[31mFailed to connect: ${error.message}\x1b[0m`);
    }
  }

  function disconnect() {
    if (socket) {
      socket.close();
      socket = null;
    }
    if (term) {
      term.dispose();
      term = null;
    }
    currentAgentId = null;
    updateStatus('Disconnected');
  }

  function updateStatus(status) {
    const statusEl = document.getElementById('terminal-status');
    if (statusEl) {
      statusEl.textContent = status;
      statusEl.className = 'terminal-status';
      if (status === 'Connected') {
        statusEl.classList.add('connected');
      } else if (status === 'Disconnected' || status === 'Failed') {
        statusEl.classList.add('disconnected');
      }
    }
  }

  return {
    init,
    connect,
    disconnect,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = terminal;
}
