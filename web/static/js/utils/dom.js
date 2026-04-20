/**
 * DOM Utilities for Clawdock Dashboard
 */

const dom = (() => {
  function createElement(tag, options = {}) {
    const element = document.createElement(tag);

    if (options.className) {
      element.className = options.className;
    }

    if (options.classes) {
      options.classes.forEach((cls) => element.classList.add(cls));
    }

    if (options.id) {
      element.id = options.id;
    }

    if (options.text) {
      element.textContent = options.text;
    }

    if (options.html) {
      element.innerHTML = options.html;
    }

    if (options.attributes) {
      Object.entries(options.attributes).forEach(([key, value]) => {
        element.setAttribute(key, value);
      });
    }

    if (options.styles) {
      Object.assign(element.style, options.styles);
    }

    if (options.children) {
      options.children.forEach((child) => {
        if (typeof child === 'string') {
          element.appendChild(document.createTextNode(child));
        } else if (child instanceof Node) {
          element.appendChild(child);
        }
      });
    }

    if (options.onClick) {
      element.addEventListener('click', options.onClick);
    }

    if (options.onChange) {
      element.addEventListener('change', options.onChange);
    }

    if (options.onInput) {
      element.addEventListener('input', options.onInput);
    }

    if (options.onSubmit) {
      element.addEventListener('submit', options.onSubmit);
    }

    return element;
  }

  function empty(element) {
    while (element.firstChild) {
      element.removeChild(element.firstChild);
    }
  }

  function show(element) {
    element.classList.remove('hidden');
  }

  function hide(element) {
    element.classList.add('hidden');
  }

  function toggle(element, force) {
    if (force === undefined) {
      element.classList.toggle('hidden');
    } else if (force) {
      show(element);
    } else {
      hide(element);
    }
  }

  function onReady(callback) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', callback);
    } else {
      callback();
    }
  }

  function delegate(container, eventType, selector, handler) {
    container.addEventListener(eventType, (event) => {
      const target = event.target.closest(selector);
      if (target && container.contains(target)) {
        handler(event, target);
      }
    });
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function setContent(element, content) {
    empty(element);
    if (typeof content === 'string') {
      element.innerHTML = content;
    } else if (content instanceof Node) {
      element.appendChild(content);
    } else if (Array.isArray(content)) {
      content.forEach((item) => {
        if (typeof item === 'string') {
          element.appendChild(document.createTextNode(item));
        } else if (item instanceof Node) {
          element.appendChild(item);
        }
      });
    }
  }

  return {
    createElement,
    empty,
    show,
    hide,
    toggle,
    onReady,
    delegate,
    escapeHtml,
    setContent,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = dom;
}
