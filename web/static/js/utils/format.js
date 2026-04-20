/**
 * Formatting Utilities for Clawdock Dashboard
 */

const format = (() => {
  const dateFormatter = new Intl.DateTimeFormat('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

  const dateFormatterLong = new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  const relativeFormatter = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

  function date(dateString) {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    return dateFormatter.format(date);
  }

  function dateLong(dateString) {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    return dateFormatterLong.format(date);
  }

  function relative(dateString) {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    const now = new Date();
    const diffInSeconds = Math.floor((now - date) / 1000);

    if (diffInSeconds < 60) return 'Just now';
    if (diffInSeconds < 3600) return relativeFormatter.format(-Math.floor(diffInSeconds / 60), 'minute');
    if (diffInSeconds < 86400) return relativeFormatter.format(-Math.floor(diffInSeconds / 3600), 'hour');
    if (diffInSeconds < 604800) return relativeFormatter.format(-Math.floor(diffInSeconds / 86400), 'day');

    return dateFormatter.format(date);
  }

  function bytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
  }

  function duration(seconds) {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
    return `${Math.floor(seconds / 86400)}d`;
  }

  function truncate(str, length = 50) {
    if (!str || str.length <= length) return str;
    return str.substring(0, length) + '...';
  }

  function slugify(text) {
    return text
      .toString()
      .toLowerCase()
      .trim()
      .replace(/\s+/g, '-')
      .replace(/[^\w\-]+/g, '')
      .replace(/\-\-+/g, '-');
  }

  function capitalize(str) {
    if (!str) return '';
    return str.charAt(0).toUpperCase() + str.slice(1);
  }

  function providerDisplayName(id) {
    const names = {
      openai: 'OpenAI',
      anthropic: 'Anthropic',
      google: 'Google AI',
      ollama: 'Ollama',
      openrouter: 'OpenRouter',
      custom: 'Custom',
    };
    return names[id] || capitalize(id);
  }

  function statusDisplay(status) {
    const names = {
      running: 'Running',
      stopped: 'Stopped',
      error: 'Error',
      drifted: 'Drifted',
      in_sync: 'In Sync',
      unknown: 'Unknown',
    };
    return names[status] || capitalize(status);
  }

  function actionDisplay(action) {
    return action
      .replace(/_/g, ' ')
      .split(' ')
      .map(capitalize)
      .join(' ');
  }

  return {
    date,
    dateLong,
    relative,
    bytes,
    duration,
    truncate,
    slugify,
    capitalize,
    providerDisplayName,
    statusDisplay,
    actionDisplay,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = format;
}
