/* Sportss Colombia - detalle de cancha: galería, disponibilidad y reserva. */
(function () {
  'use strict';

  const doc = document;
  const Sportss = window.Sportss;
  if (!Sportss) return;

  const widget = doc.getElementById('booking-widget');

  function hourToMinutes(value) {
    const parts = String(value).split(':');
    return Number(parts[0]) * 60 + Number(parts[1] || 0);
  }

  function isContiguous(hours) {
    if (hours.length < 2) return true;
    for (let index = 1; index < hours.length; index += 1) {
      if (hourToMinutes(hours[index]) - hourToMinutes(hours[index - 1]) !== 60) {
        return false;
      }
    }
    return true;
  }

  function prettyDate(iso) {
    if (!iso) return '';
    const parts = iso.split('-').map(Number);
    if (parts.length !== 3 || parts.some(Number.isNaN)) return iso;
    const date = new Date(parts[0], parts[1] - 1, parts[2], 12, 0, 0);
    return new Intl.DateTimeFormat('es-CO', {
      weekday: 'short',
      day: 'numeric',
      month: 'short'
    }).format(date);
  }

  if (widget) {
    const grid = doc.getElementById('slot-grid');
    const dateInput = doc.getElementById('booking-date');
    const dateLabel = doc.getElementById('slots-date-label');
    const summary = doc.getElementById('booking-summary');
    const rangeLabel = doc.getElementById('summary-range');
    const totalLabel = doc.getElementById('summary-total');
    const bookButton = doc.getElementById('book-button');
    const courtId = Number(widget.dataset.courtId);
    const price = Number(widget.dataset.price) || 0;

    const slotData = new Map();
    let selectedHours = [];

    function readServerSlots() {
      if (!grid) return;
      grid.querySelectorAll('.slot').forEach(function (button) {
        slotData.set(button.dataset.hour, {
          end: button.dataset.end,
          available: !button.disabled
        });
      });
    }

    function paintSelection() {
      if (!grid) return;
      grid.querySelectorAll('.slot').forEach(function (button) {
        const selected = selectedHours.indexOf(button.dataset.hour) >= 0;
        button.classList.toggle('is-selected', selected);
        button.setAttribute('aria-pressed', selected ? 'true' : 'false');
      });

      const empty = selectedHours.length === 0;
      if (summary) summary.hidden = empty;
      if (empty) {
        if (rangeLabel) rangeLabel.textContent = '—';
        if (totalLabel) totalLabel.textContent = '—';
        if (bookButton) bookButton.disabled = true;
        return;
      }

      const first = selectedHours[0];
      const last = selectedHours[selectedHours.length - 1];
      const slot = slotData.get(last);
      const end = slot ? slot.end : '';
      if (rangeLabel) rangeLabel.textContent = first + ' – ' + end;
      if (totalLabel) totalLabel.textContent = Sportss.formatMoney(price * selectedHours.length);
      if (bookButton) bookButton.disabled = false;
    }

    function renderSlots(slots) {
      if (!grid) return;
      slotData.clear();
      selectedHours = [];
      grid.textContent = '';

      if (!slots || !slots.length) {
        const empty = doc.createElement('p');
        empty.className = 'muted small slot-empty';
        empty.textContent = 'No hay horarios disponibles para esta fecha.';
        grid.appendChild(empty);
        paintSelection();
        return;
      }

      slots.forEach(function (slot) {
        slotData.set(slot.hour, { end: slot.end, available: Boolean(slot.available) });

        const button = doc.createElement('button');
        button.type = 'button';
        button.className = 'slot' + (slot.available ? '' : ' is-busy');
        button.dataset.hour = slot.hour;
        button.dataset.end = slot.end;
        button.textContent = slot.hour;
        button.setAttribute('aria-pressed', 'false');
        if (!slot.available) {
          button.disabled = true;
          if (slot.reason) button.title = slot.reason;
        }
        grid.appendChild(button);
      });

      paintSelection();
    }

    function showSlotsMessage(message) {
      if (!grid) return;
      grid.textContent = '';
      const paragraph = doc.createElement('p');
      paragraph.className = 'muted small slot-empty';
      paragraph.textContent = message;
      grid.appendChild(paragraph);
    }

    async function loadAvailability(date) {
      if (!date) return;
      selectedHours = [];
      paintSelection();
      showSlotsMessage('Consultando disponibilidad…');

      try {
        const data = await Sportss.api(
          '/api/courts/' + courtId + '/availability?date=' + encodeURIComponent(date)
        );
        renderSlots(data.slots);
        if (dateLabel) dateLabel.textContent = prettyDate(date);
      } catch (error) {
        showSlotsMessage(error.message);
        Sportss.toast(error.message, 'error');
      }
    }

    if (grid) {
      grid.addEventListener('click', function (event) {
        const button = event.target.closest('.slot');
        if (!button || button.disabled) return;

        const hour = button.dataset.hour;
        const alreadySelected = selectedHours.indexOf(hour) >= 0;
        let next;

        if (alreadySelected) {
          next = selectedHours.filter(function (value) {
            return value !== hour;
          });
        } else {
          next = selectedHours.concat(hour).sort();
        }

        if (next.length > 1 && !isContiguous(next)) {
          next = alreadySelected ? [] : [hour];
        }

        selectedHours = next;
        paintSelection();
      });
    }

    if (dateInput) {
      dateInput.addEventListener('change', function () {
        loadAvailability(dateInput.value);
      });
    }

    if (bookButton && dateInput) {
      bookButton.addEventListener('click', async function () {
        if (!selectedHours.length) return;

        const start = selectedHours[0];
        const hours = selectedHours.length;
        const date = dateInput.value;

        try {
          Sportss.setLoading(bookButton, true, 'Creando reserva…');
          const created = await Sportss.api('/api/bookings', {
            method: 'POST',
            body: {
              court_id: courtId,
              date: date,
              start_time: start,
              hours: hours
            }
          });

          if (!created.booking || !created.booking.id) {
            throw new Error('No pudimos identificar la reserva creada.');
          }

          Sportss.setLoading(bookButton, true, 'Abriendo el pago…');
          const checkout = await Sportss.api('/api/payments/checkout/' + created.booking.id, {
            method: 'POST'
          });

          if (!checkout.checkout_url) {
            throw new Error('No recibimos la URL de pago. Intenta desde Mis reservas.');
          }
          window.location.href = checkout.checkout_url;
        } catch (error) {
          Sportss.toast(error.message, 'error');
          Sportss.setLoading(bookButton, false);
          await loadAvailability(date);
        }
      });
    }

    readServerSlots();
    paintSelection();
  }

  /* Galería de fotos */
  const galleryMain = doc.getElementById('gallery-main-img');
  if (galleryMain) {
    const thumbs = doc.querySelectorAll('[data-gallery-src]');
    thumbs.forEach(function (thumb) {
      thumb.addEventListener('click', function () {
        galleryMain.src = thumb.dataset.gallerySrc;
        thumbs.forEach(function (item) {
          item.classList.remove('is-active');
        });
        thumb.classList.add('is-active');
      });
    });
  }

  /* Abrir el modal de reseña desde el detalle de la cancha */
  const openReview = doc.getElementById('open-review');
  if (openReview) {
    openReview.addEventListener('click', function () {
      const form = doc.getElementById('review-form');
      if (form && openReview.dataset.bookingId) {
        form.dataset.bookingId = openReview.dataset.bookingId;
      }
      Sportss.openModal('review-modal');
    });
  }
})();
