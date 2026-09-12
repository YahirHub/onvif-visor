const state = {
  cameras: [],
  live: new Set(),
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];

const cameraGrid = $('#cameraGrid');
const emptyState = $('#emptyState');
const cameraCount = $('#cameraCount');
const statusText = $('#statusText');
const healthDot = $('#healthDot');
const cameraDialog = $('#cameraDialog');
const discoveryDialog = $('#discoveryDialog');

async function api(path, options = {}, retryAuth = true) {
  const response = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  });
  if (response.status === 401 && retryAuth) {
    const token = window.prompt('Este visor requiere un token de acceso:');
    if (!token) throw new Error('Autenticación cancelada');
    const session = await fetch('/api/session', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    if (!session.ok) throw new Error('Token inválido');
    return api(path, options, false);
  }
  if (!response.ok) {
    let message = `Error HTTP ${response.status}`;
    try {
      const payload = await response.json();
      if (payload.error) message = payload.error;
    } catch (_) {}
    throw new Error(message);
  }
  if (response.status === 204) return null;
  return response.json();
}

function setStatus(message, ok = true) {
  statusText.textContent = message;
  healthDot.classList.toggle('ok', ok);
  healthDot.classList.toggle('bad', !ok);
}

async function refreshAll() {
  try {
    const [cameras] = await Promise.all([
      api('/api/cameras'),
      refreshEvents(),
    ]);
    state.cameras = cameras || [];
    renderCameras();
    setStatus('Servicio activo · FFmpeg disponible', true);
  } catch (error) {
    setStatus(error.message, false);
  }
}

function renderCameras() {
  cameraGrid.replaceChildren();
  cameraCount.textContent = String(state.cameras.length);
  emptyState.hidden = state.cameras.length > 0;

  for (const camera of state.cameras) {
    const fragment = $('#cameraTemplate').content.cloneNode(true);
    const card = $('.camera-card', fragment);
    const image = $('.live-image', card);
    const placeholder = $('.live-placeholder', card);
    const startButton = $('.start-live', card);
    const stopButton = $('.stop-live', card);
    const toggle = $('.record-toggle', card);
    const postRoll = $('.postroll', card);
    const deleteButton = $('.delete-camera', card);

    $('.camera-name', card).textContent = camera.name;
    $('.camera-meta', card).textContent = [camera.manufacturer, camera.model, camera.profile_name]
      .filter(Boolean).join(' · ') || 'Perfil ONVIF';
    $('.camera-endpoint', card).textContent = camera.endpoint;
    $('.event-badge', card).textContent = camera.events_supported ? 'Eventos ONVIF' : 'Sin eventos';
    toggle.checked = Boolean(camera.event_recording);
    toggle.disabled = !camera.events_supported;
    postRoll.value = camera.post_roll_seconds || 20;
    postRoll.disabled = !camera.events_supported;

    const startLive = () => {
      const separator = camera.id.includes('?') ? '&' : '?';
      image.src = `/api/cameras/${encodeURIComponent(camera.id)}/live${separator}t=${Date.now()}`;
      image.hidden = false;
      placeholder.hidden = true;
      startButton.hidden = true;
      stopButton.hidden = false;
      state.live.add(camera.id);
    };
    const stopLive = () => {
      image.removeAttribute('src');
      image.hidden = true;
      placeholder.hidden = false;
      startButton.hidden = false;
      stopButton.hidden = true;
      state.live.delete(camera.id);
    };

    startButton.addEventListener('click', startLive);
    stopButton.addEventListener('click', stopLive);
    image.addEventListener('error', () => {
      stopLive();
      setStatus(`No se pudo abrir el vídeo de ${camera.name}`, false);
    });

    async function saveRecordingSettings() {
      const seconds = Math.max(1, Math.min(600, Number(postRoll.value) || 20));
      postRoll.value = String(seconds);
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}/recording`, {
          method: 'PUT',
          body: JSON.stringify({ enabled: toggle.checked, post_roll_seconds: seconds }),
        });
        setStatus(`Configuración de ${camera.name} actualizada`, true);
      } catch (error) {
        toggle.checked = !toggle.checked;
        setStatus(error.message, false);
      }
    }
    toggle.addEventListener('change', saveRecordingSettings);
    postRoll.addEventListener('change', saveRecordingSettings);

    deleteButton.addEventListener('click', async () => {
      if (!window.confirm(`¿Eliminar ${camera.name}?`)) return;
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}`, { method: 'DELETE' });
        state.live.delete(camera.id);
        await refreshAll();
      } catch (error) {
        setStatus(error.message, false);
      }
    });

    if (state.live.has(camera.id)) startLive();
    cameraGrid.append(fragment);
  }
}

async function refreshEvents() {
  try {
    const events = await api('/api/events');
    renderEvents(events || []);
  } catch (error) {
    if (!String(error.message).includes('Autenticación cancelada')) {
      setStatus(error.message, false);
    }
  }
}

function renderEvents(events) {
  const list = $('#eventsList');
  list.replaceChildren();
  if (!events.length) {
    const empty = document.createElement('p');
    empty.className = 'hint';
    empty.textContent = 'Todavía no se han recibido eventos ONVIF.';
    list.append(empty);
    return;
  }
  for (const event of events.slice(0, 60)) {
    const row = document.createElement('div');
    row.className = 'event-row';
    const camera = document.createElement('strong');
    camera.textContent = event.camera_name;
    const topic = document.createElement('span');
    topic.className = 'event-topic mono';
    topic.textContent = event.topic || 'Evento ONVIF';
    const right = document.createElement('div');
    const time = document.createElement('div');
    time.className = 'event-time';
    time.textContent = new Date(event.time).toLocaleString();
    right.append(time);
    if (event.recording_trigger) {
      const trigger = document.createElement('div');
      trigger.className = 'event-trigger';
      trigger.textContent = '● dispara grabación';
      right.append(trigger);
    }
    row.append(camera, topic, right);
    list.append(row);
  }
}

async function discover() {
  const button = $('#discoverBtn');
  const status = $('#discoveryStatus');
  const list = $('#discoveryList');
  button.disabled = true;
  list.replaceChildren();
  status.textContent = 'Buscando cámaras ONVIF en la red local…';
  discoveryDialog.showModal();
  try {
    const devices = await api('/api/discover', { method: 'POST', body: '{}' });
    if (!devices.length) {
      status.textContent = 'No se detectaron dispositivos. Puedes agregar uno manualmente.';
      return;
    }
    status.textContent = `${devices.length} dispositivo(s) detectado(s).`;
    for (const device of devices) {
      const item = document.createElement('div');
      item.className = 'discovery-item';
      const details = document.createElement('div');
      const name = document.createElement('p');
      const endpoint = document.createElement('p');
      name.textContent = device.name || 'Cámara ONVIF';
      endpoint.className = 'mono hint';
      endpoint.textContent = device.endpoint;
      details.append(name, endpoint);
      if (device.location) {
        const location = document.createElement('p');
        location.className = 'hint';
        location.textContent = device.location;
        details.append(location);
      }
      const add = document.createElement('button');
      add.textContent = 'Agregar';
      add.addEventListener('click', () => {
        $('#cameraName').value = device.name || '';
        $('#cameraEndpoint').value = device.endpoint;
        discoveryDialog.close();
        cameraDialog.showModal();
        $('#cameraUsername').focus();
      });
      item.append(details, add);
      list.append(item);
    }
  } catch (error) {
    status.textContent = error.message;
  } finally {
    button.disabled = false;
  }
}

$('#cameraForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  const errorBox = $('#cameraFormError');
  const saveButton = $('#saveCameraBtn');
  errorBox.textContent = '';
  saveButton.disabled = true;
  try {
    await api('/api/cameras', {
      method: 'POST',
      body: JSON.stringify({
        name: $('#cameraName').value.trim(),
        endpoint: $('#cameraEndpoint').value.trim(),
        username: $('#cameraUsername').value,
        password: $('#cameraPassword').value,
      }),
    });
    cameraDialog.close();
    $('#cameraForm').reset();
    await refreshAll();
  } catch (error) {
    errorBox.textContent = error.message;
  } finally {
    saveButton.disabled = false;
  }
});

$('#addBtn').addEventListener('click', () => cameraDialog.showModal());
$('#discoverBtn').addEventListener('click', discover);
$('#refreshBtn').addEventListener('click', refreshAll);
$('#eventsRefreshBtn').addEventListener('click', refreshEvents);

for (const button of $$('[data-close]')) {
  button.addEventListener('click', () => {
    const dialog = document.getElementById(button.dataset.close);
    if (dialog?.open) dialog.close();
  });
}

window.addEventListener('beforeunload', () => {
  for (const image of $$('.live-image')) image.removeAttribute('src');
});

refreshAll();
setInterval(refreshEvents, 10000);
