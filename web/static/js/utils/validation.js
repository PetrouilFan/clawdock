/**
 * Validation Utilities for Clawdock Dashboard
 */

const validation = (() => {
  function isRequired(value) {
    if (value === null || value === undefined) return false;
    if (typeof value === 'string') return value.trim().length > 0;
    if (Array.isArray(value)) return value.length > 0;
    if (typeof value === 'object') return Object.keys(value).length > 0;
    return true;
  }

  function isString(value) {
    return typeof value === 'string';
  }

  function isNumber(value) {
    return typeof value === 'number' && !isNaN(value);
  }

  function isInteger(value) {
    return isNumber(value) && Number.isInteger(value);
  }

  function isPositive(value) {
    return isNumber(value) && value > 0;
  }

  function minLength(value, min) {
    if (!value) return false;
    return value.length >= min;
  }

  function maxLength(value, max) {
    if (!value) return true;
    return value.length <= max;
  }

  function isEmail(value) {
    if (!value) return false;
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(value);
  }

  function isURL(value) {
    if (!value) return false;
    try {
      new URL(value);
      return true;
    } catch {
      return false;
    }
  }

  function isPath(value) {
    if (!value) return false;
    // Basic path validation - should start with / and not contain invalid chars
    return /^\/[^<>|:"*?]+$/.test(value);
  }

  function isJSON(value) {
    if (!value) return true; // Empty is valid
    try {
      JSON.parse(value);
      return true;
    } catch {
      return false;
    }
  }

  function isSlug(value) {
    if (!value) return false;
    return /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value);
  }

  function matches(value, pattern) {
    if (!value) return false;
    return pattern.test(value);
  }

  function oneOf(value, options) {
    return options.includes(value);
  }

  function validateAgent(data) {
    const errors = {};

    if (!isRequired(data.name)) {
      errors.name = 'Name is required';
    } else if (!minLength(data.name, 2)) {
      errors.name = 'Name must be at least 2 characters';
    } else if (!maxLength(data.name, 100)) {
      errors.name = 'Name must be less than 100 characters';
    }

    if (!isRequired(data.provider_id)) {
      errors.provider_id = 'Provider is required';
    }

    if (!isRequired(data.model_id)) {
      errors.model_id = 'Model is required';
    }

    if (data.workspace_host_path && !isPath(data.workspace_host_path)) {
      errors.workspace_host_path = 'Please enter a valid absolute path';
    }

    if (data.extra_env_json && !isJSON(data.extra_env_json)) {
      errors.extra_env_json = 'Please enter valid JSON';
    }

    if (data.command_override_json && !isJSON(data.command_override_json)) {
      errors.command_override_json = 'Please enter valid JSON';
    }

    return {
      isValid: Object.keys(errors).length === 0,
      errors,
    };
  }

  function validateBackup(data) {
    const errors = {};

    if (!isRequired(data.backup_type)) {
      errors.backup_type = 'Backup type is required';
    } else if (!oneOf(data.backup_type, ['config_only', 'workspace_only', 'full'])) {
      errors.backup_type = 'Invalid backup type';
    }

    return {
      isValid: Object.keys(errors).length === 0,
      errors,
    };
  }

  function validateClone(data) {
    const errors = {};

    if (!isRequired(data.name)) {
      errors.name = 'Name is required';
    } else if (!minLength(data.name, 2)) {
      errors.name = 'Name must be at least 2 characters';
    }

    if (data.workspace_host_path && !isPath(data.workspace_host_path)) {
      errors.workspace_host_path = 'Please enter a valid absolute path';
    }

    return {
      isValid: Object.keys(errors).length === 0,
      errors,
    };
  }

  return {
    isRequired,
    isString,
    isNumber,
    isInteger,
    isPositive,
    minLength,
    maxLength,
    isEmail,
    isURL,
    isPath,
    isJSON,
    isSlug,
    matches,
    oneOf,
    validateAgent,
    validateBackup,
    validateClone,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = validation;
}
