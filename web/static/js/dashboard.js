/* Sportss Colombia - gráficas del panel y reportes. */
(function () {
  'use strict';

  const doc = document;
  const canvases = doc.querySelectorAll('canvas[data-chart]');
  if (!canvases.length || typeof window.Chart === 'undefined') return;

  const Chart = window.Chart;
  const cop = new Intl.NumberFormat('es-CO', {
    style: 'currency',
    currency: 'COP',
    maximumFractionDigits: 0
  });

  Chart.defaults.font.family = '"Inter", system-ui, sans-serif';
  Chart.defaults.color = '#667875';
  Chart.defaults.borderColor = 'rgba(16, 32, 29, .08)';

  function readData(canvas) {
    try {
      return JSON.parse(canvas.dataset.chart || '{}');
    } catch (error) {
      return {};
    }
  }

  function baseOptions() {
    return {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: '#10201d',
          padding: 10,
          cornerRadius: 8,
          displayColors: false
        }
      },
      scales: {
        x: {
          grid: { display: false },
          ticks: { maxRotation: 0, autoSkip: true, maxTicksLimit: 8 }
        },
        y: {
          beginAtZero: true,
          grid: { color: 'rgba(16, 32, 29, .06)' }
        }
      }
    };
  }

  const bookingsCanvas = doc.getElementById('bookings-chart');
  if (bookingsCanvas) {
    const data = readData(bookingsCanvas);
    const options = baseOptions();
    options.scales.y.ticks = { precision: 0 };
    options.plugins.tooltip.callbacks = {
      label: function (context) {
        const value = context.parsed.y;
        return value + (value === 1 ? ' reserva' : ' reservas');
      }
    };

    new Chart(bookingsCanvas, {
      type: 'line',
      data: {
        labels: data.labels || [],
        datasets: [
          {
            label: 'Reservas',
            data: data.counts || [],
            borderColor: '#0f766e',
            backgroundColor: 'rgba(15, 118, 110, .12)',
            pointBackgroundColor: '#0f766e',
            pointBorderColor: '#ffffff',
            pointBorderWidth: 2,
            pointRadius: 3,
            pointHoverRadius: 5,
            borderWidth: 2.5,
            fill: true,
            tension: 0.35
          }
        ]
      },
      options: options
    });
  }

  const courtsCanvas = doc.getElementById('courts-chart');
  if (courtsCanvas) {
    const data = readData(courtsCanvas);
    const options = baseOptions();
    options.scales.y.ticks = {
      callback: function (value) {
        return cop.format(value);
      }
    };
    options.plugins.tooltip.callbacks = {
      label: function (context) {
        return cop.format(context.parsed.y);
      }
    };

    new Chart(courtsCanvas, {
      type: 'bar',
      data: {
        labels: data.courtLabels || [],
        datasets: [
          {
            label: 'Ingresos',
            data: data.courtRevenue || [],
            backgroundColor: '#f59e0b',
            hoverBackgroundColor: '#d97706',
            borderRadius: 7,
            maxBarThickness: 44
          }
        ]
      },
      options: options
    });
  }
})();
