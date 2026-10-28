/* Sportss Colombia - formulario de publicación/edición de canchas. */
(function () {
  'use strict';

  const doc = document;
  const Sportss = window.Sportss;
  const form = doc.getElementById('court-form');
  if (!form || !Sportss) return;

  const sportSelect = form.querySelector('[data-sport-select]');
  const surfaceSelect = form.querySelector('[data-surface-select]');
  const lengthInput = doc.getElementById('length');
  const widthInput = doc.getElementById('width');
  const dimensionHint = doc.getElementById('dimension-hint');
  const latitudeInput = doc.getElementById('latitude');
  const longitudeInput = doc.getElementById('longitude');
  const mapWrap = doc.getElementById('form-map-wrap');
  const mapFrame = doc.getElementById('form-map');
  const photosInput = doc.getElementById('photos');

  let surfaces = {};
  try {
    surfaces = JSON.parse(form.dataset.surfaces || '{}');
  } catch (error) {
    surfaces = {};
  }

  const dimensions = {
    'Fútbol 5': [40, 20],
    'Fútbol 7': [50, 30],
    'Fútbol 11': [100, 64],
    'Fútbol sala': [40, 20],
    Microfútbol: [40, 20],
    Baloncesto: [28, 15],
    Voleibol: [18, 9],
    Tenis: [23.77, 10.97],
    Pádel: [20, 10],
    'Béisbol/Softbol': [100, 100],
    Otro: [40, 20]
  };

  function fillSurfaces(sport, keepValue) {
    if (!surfaceSelect) return;
    const options = surfaces[sport] || [];
    const previous = keepValue ? surfaceSelect.value : '';
    surfaceSelect.textContent = '';

    if (!options.length) {
      const option = doc.createElement('option');
      option.value = '';
      option.textContent = 'Primero elige un deporte';
      surfaceSelect.appendChild(option);
      return;
    }

    options.forEach(function (surface, index) {
      const option = doc.createElement('option');
      option.value = surface;
      option.textContent = surface;
      if (surface === previous || (!previous && index === 0)) {
        option.selected = true;
      }
      surfaceSelect.appendChild(option);
    });
  }

  function applySport(sport, useSuggestedDimensions, keepValue) {
    fillSurfaces(sport, Boolean(keepValue));

    const suggested = dimensions[sport];
    if (dimensionHint) {
      dimensionHint.textContent = suggested
        ? 'Medida estándar sugerida: ' + suggested[0] + ' × ' + suggested[1] + ' m.'
        : 'Ingresa las medidas de tu cancha en metros.';
    }

    if (useSuggestedDimensions && suggested) {
      if (lengthInput && !lengthInput.value) lengthInput.value = suggested[0];
      if (widthInput && !widthInput.value) widthInput.value = suggested[1];
    }
  }

  if (sportSelect) {
    sportSelect.addEventListener('change', function () {
      applySport(sportSelect.value, true, false);
    });

    if (form.dataset.edit === 'true' && sportSelect.value) {
      applySport(sportSelect.value, false, true);
    }
  }

  /* Vista previa del mapa a partir de coordenadas. */
  function updateMap() {
    if (!mapFrame || !mapWrap) return;
    const lat = Number(latitudeInput ? latitudeInput.value : 0);
    const lng = Number(longitudeInput ? longitudeInput.value : 0);

    const valid =
      Number.isFinite(lat) &&
      Number.isFinite(lng) &&
      Math.abs(lat) <= 90 &&
      Math.abs(lng) <= 180 &&
      (lat !== 0 || lng !== 0);

    if (!valid) {
      mapWrap.hidden = true;
      mapFrame.removeAttribute('src');
      return;
    }

    const bbox = [
      (lng - 0.006).toFixed(5),
      (lat - 0.004).toFixed(5),
      (lng + 0.006).toFixed(5),
      (lat + 0.004).toFixed(5)
    ].join(',');

    mapFrame.src =
      'https://www.openstreetmap.org/export/embed.html?bbox=' +
      encodeURIComponent(bbox) +
      '&layer=mapnik&marker=' +
      encodeURIComponent(lat + ',' + lng);
    mapWrap.hidden = false;
  }

  if (latitudeInput && longitudeInput) {
    [latitudeInput, longitudeInput].forEach(function (input) {
      input.addEventListener('change', updateMap);
      input.addEventListener('blur', updateMap);
    });
    updateMap();
  }

  /* Validación previa de fotos. */
  if (photosInput) {
    photosInput.addEventListener('change', function () {
      const files = Array.prototype.slice.call(photosInput.files || []);
      if (!files.length) return;

      const existing = form.querySelectorAll('.photo-item').length;
      if (files.length + existing > 5) {
        Sportss.toast('Una cancha puede tener máximo 5 fotos en total.', 'error');
        photosInput.value = '';
        return;
      }

      for (let index = 0; index < files.length; index += 1) {
        const file = files[index];
        if (['image/jpeg', 'image/png', 'image/webp'].indexOf(file.type) < 0) {
          Sportss.toast('Solo puedes subir imágenes JPG, PNG o WebP.', 'error');
          photosInput.value = '';
          return;
        }
        if (file.size > 5 * 1024 * 1024) {
          Sportss.toast('Cada imagen debe pesar máximo 5 MB.', 'error');
          photosInput.value = '';
          return;
        }
      }
    });
  }
})();
