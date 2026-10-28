/* Sportss Colombia - acciones de cuenta: pagos, cancelaciones y reseñas. */
(function () {
  'use strict';

  const doc = document;
  const Sportss = window.Sportss;
  if (!Sportss) return;

  /* ------------------------------------------------------------------ *
   * Checkout
   * ------------------------------------------------------------------ */

  async function startCheckout(bookingId, button) {
    if (!bookingId) return;
    try {
      Sportss.setLoading(button, true, 'Abriendo el pago…');
      const data = await Sportss.api('/api/payments/checkout/' + bookingId, { method: 'POST' });
      if (!data.checkout_url) {
        throw new Error('No recibimos la URL de pago. Intenta de nuevo.');
      }
      window.location.href = data.checkout_url;
    } catch (error) {
      Sportss.toast(error.message, 'error');
      Sportss.setLoading(button, false);
    }
  }

  doc.querySelectorAll('[data-checkout]').forEach(function (button) {
    button.addEventListener('click', function () {
      startCheckout(button.dataset.checkout, button);
    });
  });

  /* ------------------------------------------------------------------ *
   * Cancelación de reservas
   * ------------------------------------------------------------------ */

  doc.querySelectorAll('[data-cancel-booking]').forEach(function (button) {
    button.addEventListener('click', async function () {
      const bookingId = button.dataset.cancelBooking;
      if (!bookingId) return;
      const confirmed = window.confirm(
        '¿Cancelar esta reserva? El horario quedará libre para otros jugadores.'
      );
      if (!confirmed) return;

      try {
        Sportss.setLoading(button, true, 'Cancelando…');
        await Sportss.api('/api/bookings/' + bookingId + '/cancel', { method: 'POST' });
        Sportss.toast('Reserva cancelada. El horario fue liberado.', 'success');
        window.setTimeout(function () {
          window.location.reload();
        }, 650);
      } catch (error) {
        Sportss.toast(error.message, 'error');
        Sportss.setLoading(button, false);
      }
    });
  });

  /* ------------------------------------------------------------------ *
   * Reseñas
   * ------------------------------------------------------------------ */

  const reviewForm = doc.getElementById('review-form');

  function prepareReviewModal(bookingId) {
    if (!reviewForm) return;
    if (bookingId) {
      reviewForm.dataset.bookingId = bookingId;
      const hidden = doc.getElementById('review-booking-id');
      if (hidden) hidden.value = bookingId;
    }
    Sportss.openModal('review-modal');
  }

  doc.querySelectorAll('[data-review-booking]').forEach(function (button) {
    button.addEventListener('click', function () {
      prepareReviewModal(button.dataset.reviewBooking);
    });
  });

  if (reviewForm) {
    reviewForm.addEventListener('submit', async function (event) {
      event.preventDefault();

      const hidden = doc.getElementById('review-booking-id');
      const bookingId = Number(reviewForm.dataset.bookingId || (hidden ? hidden.value : 0));
      const selectedRating = reviewForm.querySelector('input[name="rating"]:checked');
      const comment = reviewForm.querySelector('textarea[name="comment"]');
      const submitButton = reviewForm.querySelector('button[type="submit"]');

      if (!bookingId) {
        Sportss.toast('No pudimos identificar la reserva que quieres calificar.', 'error');
        return;
      }
      if (!selectedRating) {
        Sportss.toast('Selecciona una calificación de 1 a 5 estrellas.', 'error');
        return;
      }
      if (!comment || !comment.value.trim()) {
        Sportss.toast('Escribe un comentario sobre tu experiencia.', 'error');
        if (comment) comment.focus();
        return;
      }

      try {
        Sportss.setLoading(submitButton, true, 'Publicando…');
        await Sportss.api('/api/reviews', {
          method: 'POST',
          body: {
            booking_id: bookingId,
            rating: Number(selectedRating.value),
            comment: comment.value.trim()
          }
        });
        Sportss.closeModal('review-modal');
        Sportss.toast('¡Gracias por tu reseña!', 'success');
        window.setTimeout(function () {
          window.location.reload();
        }, 700);
      } catch (error) {
        Sportss.toast(error.message, 'error');
        Sportss.setLoading(submitButton, false);
      }
    });
  }

  /* ------------------------------------------------------------------ *
   * Seguimiento de pagos pendientes
   * ------------------------------------------------------------------ */

  const pending = doc.querySelector('[data-pending-booking]');
  if (pending) {
    const bookingId = pending.dataset.pendingBooking;
    let attempts = 0;
    let failures = 0;

    const timer = window.setInterval(async function () {
      attempts += 1;
      if (attempts > 48) {
        window.clearInterval(timer);
        return;
      }

      try {
        const data = await Sportss.api('/api/bookings/' + bookingId);
        failures = 0;
        if (data.booking && data.booking.status && data.booking.status !== 'pendiente_pago') {
          window.clearInterval(timer);
          window.location.reload();
        }
      } catch (error) {
        failures += 1;
        if (failures >= 4) window.clearInterval(timer);
      }
    }, 5000);
  }

  /* ------------------------------------------------------------------ *
   * Cambio de contraseña
   * ------------------------------------------------------------------ */

  const newPassword = doc.getElementById('new-password');
  const confirmNewPassword = doc.getElementById('confirm-new-password');
  if (newPassword && confirmNewPassword) {
    const passwordForm = newPassword.closest('form');
    if (passwordForm) {
      passwordForm.addEventListener('submit', function (event) {
        if (newPassword.value !== confirmNewPassword.value) {
          event.preventDefault();
          Sportss.toast('Las contraseñas nuevas no coinciden.', 'error');
          confirmNewPassword.focus();
        }
      });
    }
  }
})();
