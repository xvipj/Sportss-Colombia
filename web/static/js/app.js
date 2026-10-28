/* Sportss Colombia - interacciones globales (sin scripts inline por CSP). */
(function () {
  'use strict';

  const doc = document;
  const Sportss = (window.Sportss = window.Sportss || {});

  /* ------------------------------------------------------------------ *
   * Utilidades compartidas
   * ------------------------------------------------------------------ */

  function csrfToken() {
    const match = doc.cookie.match(/(?:^|;\s*)sportss_csrf=([^;]+)/);
    return match ? decodeURIComponent(match[1]) : '';
  }

  async function api(url, options) {
    const opts = Object.assign({ credentials: 'same-origin' }, options || {});
    opts.headers = Object.assign({ Accept: 'application/json' }, opts.headers || {});

    if (opts.body && typeof opts.body !== 'string' && !(opts.body instanceof FormData)) {
      opts.body = JSON.stringify(opts.body);
      opts.headers['Content-Type'] = 'application/json';
    }
    opts.headers['X-CSRF-Token'] = csrfToken();

    let response;
    try {
      response = await fetch(url, opts);
    } catch (error) {
      throw new Error('No pudimos conectar con el servidor. Revisa tu conexión.');
    }

    let payload = null;
    try {
      payload = await response.json();
    } catch (error) {
      payload = null;
    }

    if (!response.ok) {
      const message = (payload && payload.error) || 'Ocurrió un error inesperado. Intenta de nuevo.';
      const failure = new Error(message);
      failure.status = response.status;
      throw failure;
    }
    return payload || {};
  }

  const moneyFormatter = new Intl.NumberFormat('es-CO', {
    style: 'currency',
    currency: 'COP',
    maximumFractionDigits: 0
  });

  function formatMoney(value) {
    return moneyFormatter.format(Number(value) || 0);
  }

  function formatDate(value, withTime) {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    const options = withTime
      ? { dateStyle: 'medium', timeStyle: 'short' }
      : { dateStyle: 'medium' };
    return new Intl.DateTimeFormat('es-CO', options).format(date);
  }

  function setLoading(button, loading, label) {
    if (!button) return;
    if (!button.dataset.originalHtml) {
      button.dataset.originalHtml = button.innerHTML;
    }
    button.disabled = loading;
    button.classList.toggle('is-loading', Boolean(loading));
    if (loading) {
      button.textContent = label || 'Procesando…';
    } else {
      button.innerHTML = button.dataset.originalHtml;
    }
  }

  Sportss.api = api;
  Sportss.csrf = csrfToken;
  Sportss.formatMoney = formatMoney;
  Sportss.formatDate = formatDate;
  Sportss.setLoading = setLoading;

  /* ------------------------------------------------------------------ *
   * Toasts
   * ------------------------------------------------------------------ */

  function removeToast(element) {
    if (!element || !element.parentNode) return;
    element.parentNode.removeChild(element);
  }

  function toast(message, type, timeout) {
    const container = doc.getElementById('toast-container');
    if (!container || !message) return;

    const element = doc.createElement('div');
    element.className = 'toast toast-' + (type || 'success');
    element.setAttribute('role', type === 'error' ? 'alert' : 'status');

    const text = doc.createElement('span');
    text.textContent = message;

    const close = doc.createElement('button');
    close.type = 'button';
    close.className = 'toast-close';
    close.setAttribute('aria-label', 'Cerrar mensaje');
    close.innerHTML = '&times;';
    close.addEventListener('click', function () {
      removeToast(element);
    });

    element.appendChild(text);
    element.appendChild(close);
    container.appendChild(element);

    window.setTimeout(function () {
      removeToast(element);
    }, timeout || 5200);
  }

  Sportss.toast = toast;

  doc.querySelectorAll('.toast[data-dismissible]').forEach(function (element) {
    const close = element.querySelector('.toast-close');
    if (close) {
      close.addEventListener('click', function () {
        removeToast(element);
      });
    }
    window.setTimeout(function () {
      removeToast(element);
    }, 6500);
  });

  /* ------------------------------------------------------------------ *
   * Navegación móvil y desplegables
   * ------------------------------------------------------------------ */

  const navToggle = doc.getElementById('nav-toggle');
  const primaryNav = doc.getElementById('primary-nav');

  if (navToggle && primaryNav) {
    navToggle.addEventListener('click', function () {
      const open = primaryNav.classList.toggle('is-open');
      navToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      navToggle.setAttribute('aria-label', open ? 'Cerrar menú' : 'Abrir menú');
    });

    primaryNav.querySelectorAll('a').forEach(function (link) {
      link.addEventListener('click', function () {
        primaryNav.classList.remove('is-open');
        navToggle.setAttribute('aria-expanded', 'false');
      });
    });
  }

  function closeDropdowns() {
    doc.querySelectorAll('.notif-panel, .user-menu').forEach(function (panel) {
      panel.hidden = true;
    });
    doc.querySelectorAll('#notif-toggle, #user-toggle').forEach(function (button) {
      button.setAttribute('aria-expanded', 'false');
    });
  }

  function setupDropdown(buttonId, panelId, onOpen) {
    const button = doc.getElementById(buttonId);
    const panel = doc.getElementById(panelId);
    if (!button || !panel) return;

    button.addEventListener('click', function (event) {
      event.stopPropagation();
      const willOpen = panel.hidden;
      closeDropdowns();
      if (willOpen) {
        panel.hidden = false;
        button.setAttribute('aria-expanded', 'true');
        if (typeof onOpen === 'function') onOpen();
      }
    });
  }

  /* ------------------------------------------------------------------ *
   * Notificaciones (lista + SSE)
   * ------------------------------------------------------------------ */

  const notifBadge = doc.getElementById('notif-badge');
  const notifList = doc.getElementById('notif-list');
  const notifPanel = doc.getElementById('notif-panel');
  const notifToggle = doc.getElementById('notif-toggle');
  const notifReadAll = doc.getElementById('notif-read-all');
  let notificationsLoaded = false;

  function updateBadge(count) {
    if (!notifBadge) return;
    const value = Number(count) || 0;
    notifBadge.textContent = value > 99 ? '99+' : String(value);
    notifBadge.classList.toggle('is-hidden', value < 1);
  }

  function renderNotifications(items) {
    if (!notifList) return;
    notifList.textContent = '';

    if (!items || !items.length) {
      const empty = doc.createElement('p');
      empty.className = 'muted small pad';
      empty.textContent = 'No tienes notificaciones por ahora.';
      notifList.appendChild(empty);
      return;
    }

    items.forEach(function (notification) {
      const item = doc.createElement('button');
      item.type = 'button';
      item.className = 'notif-item' + (notification.read ? '' : ' unread');

      const title = doc.createElement('strong');
      title.textContent = notification.title || 'Notificación';

      const message = doc.createElement('span');
      message.textContent = notification.message || '';

      const time = doc.createElement('time');
      time.dateTime = notification.created_at || '';
      time.textContent = formatDate(notification.created_at, true);

      item.appendChild(title);
      item.appendChild(message);
      item.appendChild(time);

      item.addEventListener('click', async function () {
        try {
          if (!notification.read) {
            await api('/api/notifications/' + notification.id + '/read', { method: 'POST' });
            notification.read = true;
            item.classList.remove('unread');
            const unread = Number(notifBadge ? notifBadge.textContent : 0);
            updateBadge(Math.max(0, unread - 1));
          }
        } catch (error) {
          toast(error.message, 'error');
          return;
        }
        if (notification.link) {
          window.location.href = notification.link;
        }
      });

      notifList.appendChild(item);
    });
  }

  async function loadNotifications() {
    if (!notifList) return;
    if (!notificationsLoaded) {
      notifList.textContent = '';
      const loading = doc.createElement('p');
      loading.className = 'muted small pad';
      loading.textContent = 'Cargando…';
      notifList.appendChild(loading);
    }
    try {
      const data = await api('/api/notifications');
      notificationsLoaded = true;
      renderNotifications(data.notifications || []);
      updateBadge(data.unread);
    } catch (error) {
      notifList.textContent = '';
      const failure = doc.createElement('p');
      failure.className = 'muted small pad';
      failure.textContent = 'No pudimos cargar tus notificaciones.';
      notifList.appendChild(failure);
    }
  }

  if (notifToggle) {
    setupDropdown('notif-toggle', 'notif-panel', loadNotifications);

    if (notifReadAll) {
      notifReadAll.addEventListener('click', async function (event) {
        event.stopPropagation();
        try {
          await api('/api/notifications/read-all', { method: 'POST' });
          updateBadge(0);
          await loadNotifications();
          toast('Notificaciones marcadas como leídas.', 'success');
        } catch (error) {
          toast(error.message, 'error');
        }
      });
    }

    if (window.EventSource) {
      try {
        const stream = new EventSource('/api/notifications/stream');
        stream.addEventListener('notification', function (event) {
          let notification = null;
          try {
            notification = JSON.parse(event.data);
          } catch (error) {
            notification = null;
          }
          if (!notification) return;
          const current = Number(notifBadge ? notifBadge.textContent : 0) || 0;
          updateBadge(current + 1);
          toast(notification.title || 'Nueva notificación', 'success');
          if (!notifList || (notifPanel && !notifPanel.hidden)) {
            loadNotifications();
          } else {
            notificationsLoaded = false;
          }
        });
      } catch (error) {
        /* El navegador reintentará solo; la app funciona sin SSE. */
      }
    }
  }

  setupDropdown('user-toggle', 'user-menu');

  doc.addEventListener('click', function (event) {
    if (!event.target.closest('.notif-wrap') && !event.target.closest('.user-wrap')) {
      closeDropdowns();
    }
  });

  doc.addEventListener('keydown', function (event) {
    if (event.key === 'Escape') {
      closeDropdowns();
      doc.querySelectorAll('.modal:not([hidden])').forEach(function (modal) {
        Sportss.closeModal(modal.id);
      });
    }
  });

  /* ------------------------------------------------------------------ *
   * Modales
   * ------------------------------------------------------------------ */

  let lastFocusedElement = null;

  function openModal(id) {
    const modal = typeof id === 'string' ? doc.getElementById(id) : id;
    if (!modal) return;
    lastFocusedElement = doc.activeElement;
    modal.hidden = false;
    doc.body.style.overflow = 'hidden';
    const focusable = modal.querySelector('input:not([type="hidden"]), textarea, button, [href], select');
    if (focusable) {
      window.setTimeout(function () {
        focusable.focus();
      }, 40);
    }
  }

  function closeModal(id) {
    const modal = typeof id === 'string' ? doc.getElementById(id) : id;
    if (!modal) return;
    modal.hidden = true;
    doc.body.style.overflow = '';
    if (lastFocusedElement && typeof lastFocusedElement.focus === 'function') {
      lastFocusedElement.focus();
    }
  }

  Sportss.openModal = openModal;
  Sportss.closeModal = closeModal;

  doc.querySelectorAll('.modal').forEach(function (modal) {
    modal.querySelectorAll('[data-close-modal]').forEach(function (closer) {
      closer.addEventListener('click', function () {
        closeModal(modal);
      });
    });
  });

  /* ------------------------------------------------------------------ *
   * Confirmaciones, contraseñas y botones sociales
   * ------------------------------------------------------------------ */

  doc.querySelectorAll('form[data-confirm]').forEach(function (form) {
    form.addEventListener('submit', function (event) {
      if (form.dataset.confirmed === 'true') return;
      const message = form.dataset.confirm;
      if (message && !window.confirm(message)) {
        event.preventDefault();
        return;
      }
      form.dataset.confirmed = 'true';
    });
  });

  doc.querySelectorAll('[data-password-toggle]').forEach(function (button) {
    const input = doc.getElementById(button.dataset.passwordToggle);
    if (!input) return;
    button.addEventListener('click', function () {
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      button.textContent = show ? 'Ocultar' : 'Ver';
      button.setAttribute('aria-label', show ? 'Ocultar contraseña' : 'Mostrar contraseña');
      input.focus();
    });
  });

  doc.querySelectorAll('[data-social]').forEach(function (button) {
    button.addEventListener('click', function () {
      toast('Acceso con ' + button.dataset.social + ' disponible próximamente.', 'info');
    });
  });

  /* ------------------------------------------------------------------ *
   * Registro
   * ------------------------------------------------------------------ */

  const registerForm = doc.getElementById('register-form');
  if (registerForm) {
    const radios = registerForm.querySelectorAll('[data-account-radio]');
    const businessFields = doc.getElementById('business-fields');
    const businessName = doc.getElementById('business-name');
    const phone = doc.getElementById('phone');
    const password = doc.getElementById('reg-password');
    const confirmation = doc.getElementById('confirm-password');
    const passwordError = doc.getElementById('password-error');

    function syncAccountType() {
      const selected = registerForm.querySelector('[data-account-radio]:checked');
      const isBusiness = selected && selected.value === 'empresa';
      if (businessFields) businessFields.hidden = !isBusiness;
      if (businessName) businessName.required = Boolean(isBusiness);
      if (phone) phone.required = Boolean(isBusiness);
    }

    radios.forEach(function (radio) {
      radio.addEventListener('change', syncAccountType);
    });
    syncAccountType();

    function passwordsMatch() {
      if (!password || !confirmation) return true;
      const ok = password.value === confirmation.value;
      if (passwordError) passwordError.hidden = ok;
      return ok;
    }

    if (confirmation) confirmation.addEventListener('input', passwordsMatch);
    if (password) password.addEventListener('input', passwordsMatch);

    registerForm.addEventListener('submit', function (event) {
      if (!passwordsMatch()) {
        event.preventDefault();
        if (confirmation) confirmation.focus();
      }
    });
  }

  /* ------------------------------------------------------------------ *
   * Búsqueda
   * ------------------------------------------------------------------ */

  const filtersToggle = doc.getElementById('filters-toggle');
  const filtersPanel = doc.getElementById('filters-panel');
  if (filtersToggle && filtersPanel) {
    filtersToggle.addEventListener('click', function () {
      const open = filtersPanel.classList.toggle('is-open');
      filtersToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
  }

  doc.querySelectorAll('[data-auto-submit]').forEach(function (control) {
    control.addEventListener('change', function () {
      if (!control.form) return;
      if (typeof control.form.requestSubmit === 'function') {
        control.form.requestSubmit();
      } else {
        control.form.submit();
      }
    });
  });
})();
