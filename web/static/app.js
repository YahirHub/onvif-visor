const state = {
  cameras: [],
  live: new Set(),
  auth: { required: false, authenticated: false },
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];

const bootView = $('#bootView');
const bootStatus = $('#bootStatus');
const bootSpinner = $('#bootSpinner');
const bootRetryBtn = $('#bootRetryBtn');
const loginView = $('#loginView');
const loginForm = $('#loginForm');
const loginUsername = $('#loginUsername');
const loginPassword = $('#loginPassword');
const loginMessage = $('#loginMessage');
const loginError = $('#loginError');
const loginSubmitBtn = $('#loginSubmitBtn');
const appView = $('#appView');
const logoutBtn = $('#logoutBtn');
const cameraGrid = $('#cameraGrid');
const emptyState = $('#emptyState');
const cameraCount = $('#cameraCount');
const statusText = $('#statusText');
const healthDot = $('#healthDot');
const cameraDialog = $('#cameraDialog');
const discoveryDialog = $('#discoveryDialog');
const confirmDialog = $('#confirmDialog');

let eventsTimer = null;

class SessionRequiredError extends Error {}

async function readResponsePayload(response) {
  if (response.status === 204) return null;
  try {
    return await response.json();
  } catch (_) {
    return null;
  }
}

function responseMessage(response, payload) {
  if (payload?.error) return payload.error;
  if (response.status === 400) return 'Revisa los datos e inténtalo de nuevo.';
  if (response.status === 401) return 'La sesión no es válida. Ingresa de nuevo.';
  if (response.status === 404) return 'No se encontró el recurso solicitado.';
  if (response.status >= 500) return 'El visor no pudo completar la operación.';
  return 'No se pudo completar la operación.';
}

async function fetchJSON(path, options = {}) {
  const headers = { Accept: 'application/json', ...(options.headers || {}) };
  if (options.body !== undefined && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }

  let response;
  try {
    response = await fetch(path, {
      credentials: 'same-origin',
      ...options,
      headers,
    });
  } catch (_) {
    throw new Error('No se pudo conectar con el visor. Revisa la red y vuelve a intentar.');
  }

  const payload = await readResponsePayload(response);
  return { response, payload };
}

async function api(path, options = {}) {
  const { response, payload } = await fetchJSON(path, options);
  if (response.status === 401) {
    showLogin('Tu sesión terminó. Ingresa de nuevo para continuar.');
    throw new SessionRequiredError('Sesión requerida');
  }
  if (!response.ok) throw new Error(responseMessage(response, payload));
  return payload;
}

async function getSessionStatus() {
  const { response, payload } = await fetchJSON('/api/session');
  if (!response.ok || !payload) {
    throw new Error(responseMessage(response, payload));
  }
  return payload;
}

function closeDialogs() {
  for (const dialog of $$('dialog[open]')) dialog.close('cancel');
}

function stopLiveImages() {
  for (const image of $$('.live-image')) image.removeAttribute('src');
  state.live.clear();
}

function stopEventPolling() {
  if (eventsTimer !== null) {
    clearInterval(eventsTimer);
    eventsTimer = null;
  }
}

function startEventPolling() {
  stopEventPolling();
  eventsTimer = setInterval(refreshEvents, 10000);
}

function showLogin(message = '') {
  closeDialogs();
  stopLiveImages();
  stopEventPolling();
  if (state.auth.required) state.auth.authenticated = false;
  bootView.hidden = true;
  appView.hidden = true;
  loginView.hidden = false;
  loginError.textContent = '';
  loginMessage.textContent = message;
  loginPassword.value = '';
  requestAnimationFrame(() => loginUsername.focus());
}

function showApp() {
  bootView.hidden = true;
  loginView.hidden = true;
  appView.hidden = false;
  logoutBtn.hidden = !state.auth.required;
  loginError.textContent = '';
  loginMessage.textContent = '';
  startEventPolling();
}

function showBootError(message) {
  bootView.hidden = false;
  loginView.hidden = true;
  appView.hidden = true;
  bootStatus.textContent = message;
  bootSpinner.hidden = true;
  bootRetryBtn.hidden = false;
}

async function bootstrap() {
  stopEventPolling();
  bootView.hidden = false;
  loginView.hidden = true;
  appView.hidden = true;
  bootStatus.textContent = 'Preparando la interfaz…';
  bootSpinner.hidden = false;
  bootRetryBtn.hidden = true;

  try {
    state.auth = await getSessionStatus();
    if (state.auth.required && !state.auth.authenticated) {
      showLogin();
      return;
    }
    showApp();
    await refreshAll();
  } catch (error) {
    showBootError(error.message);
  }
}

loginForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  const username = loginUsername.value.trim();
  const password = loginPassword.value;

  loginError.textContent = '';
  loginMessage.textContent = '';
  if (!username || !password) {
    loginError.textContent = 'Escribe tu usuario y contraseña.';
    return;
  }

  loginSubmitBtn.disabled = true;
  const previousLabel = loginSubmitBtn.textContent;
  loginSubmitBtn.textContent = 'Ingresando…';

  try {
    const { response, payload } = await fetchJSON('/api/session', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    if (!response.ok) {
      loginError.textContent = responseMessage(response, payload);
      return;
    }

    state.auth = { required: true, authenticated: true };
    loginForm.reset();
    showApp();
    await refreshAll();
  } catch (error) {
    loginError.textContent = error.message;
  } finally {
    loginSubmitBtn.disabled = false;
    loginSubmitBtn.textContent = previousLabel;
  }
});

logoutBtn.addEventListener('click', async () => {
  logoutBtn.disabled = true;
  try {
    await fetchJSON('/api/session', { method: 'DELETE' });
  } finally {
    logoutBtn.disabled = false;
    state.auth.authenticated = false;
    showLogin('Sesión cerrada correctamente.');
  }
});

bootRetryBtn.addEventListener('click', bootstrap);

function setStatus(message, ok = true) {
  statusText.textContent = message;
  healthDot.classList.toggle('ok', ok);
  healthDot.classList.toggle('bad', !ok);
}

async function refreshAll() {
  try {
    const cameras = await api('/api/cameras');
    state.cameras = cameras || [];
    renderCameras();
    await refreshEvents();
    if (!appView.hidden) setStatus('Servicio activo · FFmpeg disponible', true);
  } catch (error) {
    if (error instanceof SessionRequiredError) return;
    setStatus(error.message, false);
  }
}

async function checkSessionAfterMediaError(cameraName) {
  try {
    const session = await getSessionStatus();
    state.auth = session;
    if (session.required && !session.authenticated) {
      showLogin('Tu sesión terminó. Ingresa de nuevo para continuar.');
      return;
    }
    setStatus(`No se pudo abrir el vídeo de ${cameraName}`, false);
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
      image.src = `/api/cameras/${encodeURIComponent(camera.id)}/live?t=${Date.now()}`;
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
    image.addEventListener('error', async () => {
      stopLive();
      await checkSessionAfterMediaError(camera.name);
    });

    async function saveRecordingSettings() {
      const seconds = Math.max(1, Math.min(600, Number(postRoll.value) || 20));
      postRoll.value = String(seconds);
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}/recording`, {
          method: 'PUT',
          body: JSON.stringify({ enabled: toggle.checked, post_roll_seconds: seconds }),
        });
        camera.event_recording = toggle.checked;
        camera.post_roll_seconds = seconds;
        setStatus(`Configuración de ${camera.name} actualizada`, true);
      } catch (error) {
        if (error instanceof SessionRequiredError) return;
        toggle.checked = Boolean(camera.event_recording);
        postRoll.value = String(camera.post_roll_seconds || 20);
        setStatus(error.message, false);
      }
    }
    toggle.addEventListener('change', saveRecordingSettings);
    postRoll.addEventListener('change', saveRecordingSettings);

    deleteButton.addEventListener('click', async () => {
      const accepted = await confirmAction({
        title: `Eliminar ${camera.name}`,
        message: 'Esta cámara dejará de aparecer en el visor y se detendrá cualquier vista en vivo activa. Esta acción no elimina grabaciones existentes.',
        confirmLabel: 'Eliminar cámara',
      });
      if (!accepted) return;

      deleteButton.disabled = true;
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}`, { method: 'DELETE' });
        state.live.delete(camera.id);
        await refreshAll();
      } catch (error) {
        if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
      } finally {
        deleteButton.disabled = false;
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
    if (error instanceof SessionRequiredError) return;
    if (!appView.hidden) setStatus(error.message, false);
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
    if (!(error instanceof SessionRequiredError)) status.textContent = error.message;
  } finally {
    button.disabled = false;
  }
}

function confirmAction({ title, message, confirmLabel = 'Confirmar' }) {
  $('#confirmTitle').textContent = title;
  $('#confirmMessage').textContent = message;
  $('#confirmAcceptBtn').textContent = confirmLabel;
  confirmDialog.returnValue = 'cancel';
  confirmDialog.showModal();

  return new Promise((resolve) => {
    confirmDialog.addEventListener('close', () => {
      resolve(confirmDialog.returnValue === 'confirm');
    }, { once: true });
  });
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
    if (!(error instanceof SessionRequiredError)) errorBox.textContent = error.message;
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
    if (dialog?.open) dialog.close('cancel');
  });
}

window.addEventListener('beforeunload', stopLiveImages);

bootstrap();
