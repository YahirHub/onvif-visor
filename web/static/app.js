const state = {
  cameras: [],
  recordings: [],
  personSegments: [],
  analyticsAnalyses: [],
  analyticsStatus: null,
  analyticsLoading: false,
  auth: { required: false, authenticated: false },
  currentView: 'cameras',
  monitorSelected: [],
  recordingCameraID: '',
  recordingDate: '',
  snapshotRevision: Date.now(),
  pendingPlayback: null,
  sessions: [],
  profile: null,
  telegram: null,
  playback: { recording: null, offset: 0, startedAt: 0, timer: null, playing: false },
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
const sidebarDot = $('#sidebarDot');
const sidebarStatus = $('#sidebarStatus');
const viewEyebrow = $('#viewEyebrow');
const viewTitle = $('#viewTitle');
const viewSubtitle = $('#viewSubtitle');
const headerActions = $('#headerActions');
const cameraGrid = $('#cameraGrid');
const emptyState = $('#emptyState');
const cameraCount = $('#cameraCount');
const cameraSearch = $('#cameraSearch');
const cameraDialog = $('#cameraDialog');
const scheduleDialog = $('#scheduleDialog');
const discoveryDialog = $('#discoveryDialog');
const confirmDialog = $('#confirmDialog');
const monitorStage = $('#monitorStage');
const monitorEmpty = $('#monitorEmpty');
const monitorPicker = $('#monitorPicker');
const pickerList = $('#pickerList');
const pickerSearch = $('#pickerSearch');

const viewMeta = {
  cameras: { eyebrow: 'MONITOREO', title: 'Cámaras', subtitle: 'Selecciona una cámara para abrirla.' },
  monitor: { eyebrow: 'MONITOREO', title: 'Multivista', subtitle: 'Vista en vivo.' },
  recordings: { eyebrow: 'ARCHIVO', title: 'Grabaciones', subtitle: 'Historial de vídeo por cámara.' },
  events: { eyebrow: 'ACTIVIDAD', title: 'Eventos', subtitle: 'Movimiento y eventos recientes.' },
  sessions: { eyebrow: 'SEGURIDAD', title: 'Sesiones', subtitle: 'Accesos persistentes al visor.' },
  profile: { eyebrow: 'CUENTA', title: 'Perfil', subtitle: 'Usuario y contraseña del visor.' },
  integrations: { eyebrow: 'MÓDULOS', title: 'Integraciones', subtitle: 'Servicios externos y automatizaciones.' },
};

const routePathByView = {
  cameras: '/camaras',
  monitor: '/multivista',
  recordings: '/grabaciones',
  events: '/eventos',
  sessions: '/sesiones',
  profile: '/perfil',
  integrations: '/integraciones',
};

function setBrowserRoute(path, { replace = false } = {}) {
  if (!path || window.location.pathname === path) return;
  window.history[replace ? 'replaceState' : 'pushState']({}, '', path);
}

function parseCurrentRoute() {
  let path = window.location.pathname || '/';
  while (path.length > 1 && path.endsWith('/')) path = path.slice(0, -1);

  if (path === '/') return { view: 'cameras', canonicalPath: '/camaras', replace: true };
  if (path === '/camaras') return { view: 'cameras' };
  if (path === '/multivista') return { view: 'monitor' };
  if (path === '/grabaciones') return { view: 'recordings', clearRecordingCamera: true };
  if (path === '/eventos') return { view: 'events' };
  if (path === '/sesiones') return { view: 'sessions' };
  if (path === '/perfil') return { view: 'profile' };
  if (path === '/integraciones') return { view: 'integrations' };

  if (path.startsWith('/camara/')) {
    const cameraID = path.slice('/camara/'.length);
    if (cameraID && !cameraID.includes('/')) return { view: 'monitor', cameraID: decodeURIComponent(cameraID) };
  }
  if (path.startsWith('/grabaciones/')) {
    const recordingCameraID = path.slice('/grabaciones/'.length);
    if (recordingCameraID && !recordingCameraID.includes('/')) {
      return { view: 'recordings', recordingCameraID: decodeURIComponent(recordingCameraID) };
    }
  }

  return { view: 'cameras', canonicalPath: '/camaras', replace: true };
}

function applyCurrentRoute() {
  const route = parseCurrentRoute();
  if (route.cameraID) {
    if (!state.cameras.some(camera => camera.id === route.cameraID)) {
      navigate('cameras', { replace: true });
      return;
    }
    state.monitorSelected = [route.cameraID];
  }
  if (route.clearRecordingCamera) state.recordingCameraID = '';
  if (route.recordingCameraID) {
    if (!state.cameras.some(camera => camera.id === route.recordingCameraID)) {
      navigate('recordings', { replace: true });
      return;
    }
    state.recordingCameraID = route.recordingCameraID;
  }
  if (route.canonicalPath) setBrowserRoute(route.canonicalPath, { replace: route.replace });
  navigate(route.view, { fromHistory: true });
}

let eventsTimer = null;
let camerasTimer = null;

class SessionRequiredError extends Error {}

async function readResponsePayload(response) {
  if (response.status === 204) return null;
  try { return await response.json(); } catch (_) { return null; }
}

function responseMessage(response, payload) {
  if (payload?.error) return payload.error;
  if (response.status === 400) return 'Revisa los datos e inténtalo de nuevo.';
  if (response.status === 401) return 'La sesión no es válida. Ingresa de nuevo.';
  if (response.status === 404) return 'No se encontró el recurso solicitado.';
  if (response.status === 409) return 'La operación entra en conflicto con el estado actual.';
  if (response.status >= 500) return 'El visor no pudo completar la operación.';
  return 'No se pudo completar la operación.';
}

async function fetchJSON(path, options = {}) {
  const headers = { Accept: 'application/json', ...(options.headers || {}) };
  if (options.body !== undefined && !headers['Content-Type']) headers['Content-Type'] = 'application/json';

  let response;
  try {
    response = await fetch(path, { credentials: 'same-origin', ...options, headers });
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
  if (!response.ok || !payload) throw new Error(responseMessage(response, payload));
  return payload;
}

function closeDialogs() {
  for (const dialog of $$('dialog[open]')) dialog.close('cancel');
}

function stopMonitorStreams() {
  for (const image of $$('.monitor-live-image', monitorStage)) image.removeAttribute('src');
  monitorStage.replaceChildren();
}

function stopEventPolling() {
  if (eventsTimer !== null) {
    clearInterval(eventsTimer);
    eventsTimer = null;
  }
}

function stopCameraPolling() {
  if (camerasTimer !== null) {
    clearInterval(camerasTimer);
    camerasTimer = null;
  }
}

function startEventPolling() {
  stopEventPolling();
  eventsTimer = setInterval(() => {
    if (!appView.hidden && state.auth.authenticated !== false) refreshEvents(false);
  }, 2500);
}

function startCameraPolling() {
  stopCameraPolling();
  camerasTimer = setInterval(() => {
    if (appView.hidden || state.auth.authenticated === false) return;
    refreshCamerasRealtime();
    if (state.currentView === 'recordings' && state.recordingCameraID) loadRecordingAnalytics({ silent: true });
  }, 4000);
}

function setStatus(message, ok = true) {
  sidebarStatus.textContent = message;
  sidebarDot.classList.toggle('ok', ok);
  sidebarDot.classList.toggle('bad', !ok);
}

function showLogin(message = '') {
  closeDialogs();
  stopMonitorStreams();
  stopEventPolling();
  stopCameraPolling();
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
  $('#logoutMobileBtn').hidden = !state.auth.required;
  loginError.textContent = '';
  loginMessage.textContent = '';
  startEventPolling();
  startCameraPolling();
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
  stopCameraPolling();
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
    applyCurrentRoute();
  } catch (error) {
    showBootError(error.message);
  }
}

function navigate(view, { path = '', replace = false, fromHistory = false } = {}) {
  if (!viewMeta[view]) return;
  if (state.currentView === 'monitor' && view !== 'monitor') stopMonitorStreams();
  if (state.currentView === 'recordings' && view !== 'recordings') stopRecordingPlayback();
  if (view !== 'monitor') {
    monitorPicker.classList.remove('open');
    appView.classList.remove('theater-mode');
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  }
  state.currentView = view;
  appView.dataset.view = view;

  for (const item of $$('.nav-item[data-view]')) item.classList.toggle('active', item.dataset.view === view);
  for (const section of $$('.app-view')) section.hidden = section.id !== `${view}View`;

  const meta = viewMeta[view];
  viewEyebrow.textContent = meta.eyebrow;
  viewTitle.textContent = meta.title;
  viewSubtitle.textContent = meta.subtitle;
  document.title = `${meta.title} · ONVIF Visor`;
  headerActions.hidden = view !== 'cameras';

  if (!fromHistory) setBrowserRoute(path || routePathByView[view], { replace });

  if (view === 'cameras') renderCameras();
  if (view === 'monitor') {
    if (!state.monitorSelected.length && state.cameras.length) state.monitorSelected = [state.cameras[0].id];
    renderMonitor();
  }
  if (view === 'recordings') loadRecordings();
  if (view === 'events') refreshEvents(false);
  if (view === 'sessions') loadSessions();
  if (view === 'profile') loadProfile();
  if (view === 'integrations') loadTelegramStatus();
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
    applyCurrentRoute();
  } catch (error) {
    loginError.textContent = error.message;
  } finally {
    loginSubmitBtn.disabled = false;
    loginSubmitBtn.textContent = previousLabel;
  }
});

async function logout() {
  const buttons = [logoutBtn, $('#logoutMobileBtn')].filter(Boolean);
  for (const button of buttons) button.disabled = true;
  try { await fetchJSON('/api/session', { method: 'DELETE' }); }
  finally {
    for (const button of buttons) button.disabled = false;
    state.auth.authenticated = false;
    showLogin('Sesión cerrada correctamente.');
  }
}

logoutBtn.addEventListener('click', logout);
$('#logoutMobileBtn').addEventListener('click', logout);

bootRetryBtn.addEventListener('click', bootstrap);

function skeletonMarkup(lines = 2) {
  const widths = ['wide', 'medium', 'short'];
  return Array.from({ length: lines }, (_, index) => `<span class="skeleton skeleton-line ${widths[index % widths.length]}"></span>`).join('');
}

function renderCameraSkeletons(count = 4) {
  emptyState.hidden = true;
  cameraCount.textContent = '…';
  cameraGrid.innerHTML = Array.from({ length: count }, () => `
    <article class="camera-thumb-card skeleton-card" aria-hidden="true">
      <div class="skeleton skeleton-media"></div>
      <div class="skeleton-card-copy">${skeletonMarkup(2)}</div>
    </article>`).join('');
}

function renderSessionSkeletons(count = 3) {
  const list = $('#sessionsList');
  list.innerHTML = Array.from({ length: count }, () => `
    <article class="session-card skeleton-session" aria-hidden="true">
      <span class="skeleton skeleton-avatar"></span>
      <div class="skeleton-copy">${skeletonMarkup(3)}</div>
    </article>`).join('');
}

function renderEventSkeletons(count = 4) {
  const list = $('#eventsList');
  list.innerHTML = Array.from({ length: count }, () => `
    <article class="event-card skeleton-event" aria-hidden="true">
      <span class="skeleton skeleton-event-thumb"></span>
      <div class="skeleton-copy">${skeletonMarkup(3)}</div>
    </article>`).join('');
}

function renderRecordingSkeletons() {
  const cameras = $('#recordingCameraList');
  cameras.innerHTML = Array.from({ length: 3 }, () => `<div class="recording-camera-item skeleton-recording-camera" aria-hidden="true">${skeletonMarkup(2)}</div>`).join('');
  const loading = $('#recordingsLoading');
  loading.innerHTML = `
    <div class="recordings-skeleton" aria-hidden="true">
      <div class="skeleton skeleton-line medium"></div>
      <div class="skeleton skeleton-timeline"></div>
      <div class="recording-grid skeleton-recording-grid">
        ${Array.from({ length: 4 }, () => `<div class="recording-card skeleton-card"><div class="skeleton skeleton-media"></div><div class="skeleton-card-copy">${skeletonMarkup(2)}</div></div>`).join('')}
      </div>
    </div>`;
}

function renderDiscoverySkeletons(count = 4) {
  const list = $('#discoveryList');
  list.innerHTML = Array.from({ length: count }, () => `
    <div class="discovery-item skeleton-discovery" aria-hidden="true">
      <div class="skeleton-copy">${skeletonMarkup(2)}</div>
      <span class="skeleton skeleton-button"></span>
    </div>`).join('');
}

function renderLoadError(container, title, message) {
  container.replaceChildren();
  const box = document.createElement('div');
  box.className = 'placeholder-panel compact-placeholder';
  const heading = document.createElement('h2');
  heading.textContent = title;
  const copy = document.createElement('p');
  copy.textContent = message;
  box.append(heading, copy);
  container.append(box);
}

async function refreshAll() {
  try {
    if (state.currentView === 'cameras' && !state.cameras.length) renderCameraSkeletons();
    const cameras = await api('/api/cameras');
    state.cameras = cameras || [];
    state.monitorSelected = state.monitorSelected.filter(id => state.cameras.some(camera => camera.id === id));
    renderCameras();
    renderPicker();
    await refreshEvents(false);
    setStatus('Servicio activo', true);
  } catch (error) {
    if (error instanceof SessionRequiredError) return;
    if ($('.skeleton-card', cameraGrid)) {
      cameraCount.textContent = '0';
      renderLoadError(cameraGrid, 'No se pudieron cargar las cámaras', error.message);
    }
    setStatus(error.message, false);
  }
}

function cameraFingerprint(cameras) {
  return JSON.stringify((cameras || []).map(camera => ({
    id: camera.id,
    name: camera.name,
    manufacturer: camera.manufacturer,
    model: camera.model,
    endpoint: camera.endpoint,
    event_recording: camera.event_recording,
    schedule_enabled: camera.schedule_enabled,
    schedule_days: camera.schedule_days,
    schedule_start: camera.schedule_start,
    schedule_end: camera.schedule_end,
  })));
}

async function refreshCamerasRealtime() {
  try {
    const cameras = await api('/api/cameras') || [];
    if (cameraFingerprint(cameras) === cameraFingerprint(state.cameras)) {
      if (state.currentView === 'integrations' && !$('#telegramPanel').hidden) loadTelegramStatus();
      return;
    }

    const previousSelection = state.monitorSelected.join('|');
    state.cameras = cameras;
    state.monitorSelected = state.monitorSelected.filter(id => cameras.some(camera => camera.id === id));
    const selectionChanged = previousSelection !== state.monitorSelected.join('|');

    if (state.currentView === 'cameras') renderCameras();
    renderPicker();
    if (state.currentView === 'monitor' && selectionChanged) renderMonitor();
    if (state.currentView === 'recordings') renderRecordingCameraList();
    if (state.currentView === 'integrations' && !$('#telegramPanel').hidden) loadTelegramStatus();
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  }
}

function cameraMatches(camera, query) {
  if (!query) return true;
  const haystack = [camera.name, camera.manufacturer, camera.model, camera.endpoint, camera.profile_name]
    .filter(Boolean).join(' ').toLowerCase();
  return haystack.includes(query.toLowerCase());
}

function renderCameras() {
  const query = cameraSearch.value.trim();
  const cameras = state.cameras.filter(camera => cameraMatches(camera, query));
  cameraGrid.replaceChildren();
  cameraCount.textContent = String(state.cameras.length);
  emptyState.hidden = state.cameras.length > 0 || Boolean(query);

  if (!cameras.length && query) {
    const noResults = document.createElement('div');
    noResults.className = 'empty-state';
    noResults.innerHTML = '<div class="empty-icon" aria-hidden="true">⌕</div><h2>Sin resultados</h2><p>Prueba con otro nombre, modelo o endpoint.</p>';
    cameraGrid.append(noResults);
    return;
  }

  for (const camera of cameras) {
    const fragment = $('#cameraCardTemplate').content.cloneNode(true);
    const card = $('.camera-thumb-card', fragment);
    card.classList.add('loading');
    const mediaButton = $('.camera-thumb-media', card);
    const image = $('.camera-thumb-image', card);
    const fallback = $('.camera-thumb-fallback', card);
    const statusChip = $('.live-chip', card);
    const statusText = $('.camera-status-text', card);
    const scheduleButton = $('.camera-schedule', card);
    const deleteButton = $('.camera-delete', card);
    const toggle = $('.record-toggle', card);

    $('.camera-card-name', card).textContent = camera.name;
    $('.camera-card-meta', card).textContent = [camera.manufacturer, camera.model]
      .filter(Boolean).join(' · ') || 'Cámara ONVIF';
    toggle.checked = Boolean(camera.event_recording);
    scheduleButton.classList.toggle('active', Boolean(camera.schedule_enabled));
    scheduleButton.title = camera.schedule_enabled ? 'Horario activo · editar' : 'Programar grabación';

    image.alt = `Vista previa de ${camera.name}`;
    image.addEventListener('load', () => {
      card.classList.remove('loading');
      fallback.hidden = true;
      image.hidden = false;
      statusChip.classList.remove('offline');
      statusText.textContent = 'EN LÍNEA';
    });
    image.addEventListener('error', () => {
      card.classList.remove('loading');
      image.hidden = true;
      fallback.hidden = false;
      statusChip.classList.add('offline');
      statusText.textContent = 'SIN VISTA';
    });
    image.src = `/api/cameras/${encodeURIComponent(camera.id)}/snapshot?v=${state.snapshotRevision}`;

    mediaButton.addEventListener('click', () => openMonitor(camera.id));

    toggle.addEventListener('change', async () => {
      const wanted = toggle.checked;
      toggle.disabled = true;
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}/recording`, {
          method: 'PUT',
          body: JSON.stringify({ enabled: wanted, post_roll_seconds: camera.post_roll_seconds || 20 }),
        });
        camera.event_recording = wanted;
        setStatus(wanted ? `Grabación por eventos activada en ${camera.name}` : `Grabación por eventos desactivada en ${camera.name}`, true);
      } catch (error) {
        toggle.checked = !wanted;
        if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
      } finally {
        toggle.disabled = false;
      }
    });

    scheduleButton.addEventListener('click', () => openScheduleDialog(camera));

    deleteButton.addEventListener('click', async () => {
      const accepted = await confirmAction({
        title: `Eliminar ${camera.name}`,
        message: 'La cámara dejará de aparecer en el visor y se detendrán sus transmisiones activas. Las grabaciones permanecen en disco, pero dejarán de mostrarse en la biblioteca mientras la cámara no esté configurada.',
        confirmLabel: 'Eliminar cámara',
      });
      if (!accepted) return;
      deleteButton.disabled = true;
      try {
        await api(`/api/cameras/${encodeURIComponent(camera.id)}`, { method: 'DELETE' });
        state.monitorSelected = state.monitorSelected.filter(id => id !== camera.id);
        await refreshAll();
      } catch (error) {
        if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
      } finally {
        deleteButton.disabled = false;
      }
    });

    cameraGrid.append(fragment);
  }
}

function openScheduleDialog(camera) {
  $('#scheduleCameraID').value = camera.id;
  $('#scheduleCameraTitle').textContent = camera.name;
  $('#scheduleEnabled').checked = Boolean(camera.schedule_enabled);
  $('#scheduleStart').value = camera.schedule_start || '00:00';
  $('#scheduleEnd').value = camera.schedule_end || '23:59';
  $('#scheduleFormError').textContent = '';

  const configuredDays = Array.isArray(camera.schedule_days) && camera.schedule_days.length
    ? new Set(camera.schedule_days.map(Number))
    : new Set([0, 1, 2, 3, 4, 5, 6]);
  for (const input of $$('#scheduleDays input[type="checkbox"]')) {
    input.checked = configuredDays.has(Number(input.value));
  }
  syncScheduleFormState();
  scheduleDialog.showModal();
}

function syncScheduleFormState() {
  const enabled = $('#scheduleEnabled').checked;
  $('#scheduleStart').disabled = !enabled;
  $('#scheduleEnd').disabled = !enabled;
  for (const input of $$('#scheduleDays input[type="checkbox"]')) input.disabled = !enabled;
}

function openMonitor(cameraID) {
  state.monitorSelected = [cameraID];
  navigate('monitor', { path: `/camara/${encodeURIComponent(cameraID)}` });
}

function monitorLayoutClass(count) {
  return `layout-${Math.max(1, Math.min(9, count || 1))}`;
}

function monitorCompositeRatio(count) {
  if (count === 2) return 32 / 9;
  if (count === 3 || count === 5 || count === 6) return 8 / 3;
  return 16 / 9;
}

function fitMonitorStage() {
  if (state.currentView !== 'monitor') return;
  if (window.matchMedia('(max-width: 760px)').matches) {
    monitorStage.style.width = '100%';
    monitorStage.style.height = 'auto';
    return;
  }
  const host = monitorStage.parentElement;
  if (!host) return;
  const count = Math.max(1, state.monitorSelected.length);
  const ratio = monitorCompositeRatio(count);
  const availableWidth = host.clientWidth;
  const availableHeight = host.clientHeight;
  let width = availableWidth;
  let height = width / ratio;
  if (height > availableHeight) {
    height = availableHeight;
    width = height * ratio;
  }
  monitorStage.style.width = `${Math.max(1, Math.floor(width))}px`;
  monitorStage.style.height = `${Math.max(1, Math.floor(height))}px`;
}

async function toggleMonitorFullscreen() {
  const target = $('#monitorView');
  if (!target) return;
  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
      return;
    }
    if (target.requestFullscreen) {
      await target.requestFullscreen();
      return;
    }
  } catch (_) {}
  appView.classList.toggle('theater-mode');
  syncFullscreenUI();
}

function syncFullscreenUI() {
  const active = Boolean(document.fullscreenElement) || appView.classList.contains('theater-mode');
  const button = $('#monitorFullscreenBtn');
  if (button) {
    button.textContent = active ? '⛶' : '⛶';
    button.title = active ? 'Salir de pantalla completa' : 'Pantalla completa';
    button.setAttribute('aria-label', button.title);
  }
  requestAnimationFrame(fitMonitorStage);
}

function createMonitorPane(camera, lowBandwidth) {
  const fragment = $('#monitorPaneTemplate').content.cloneNode(true);
  const pane = $('.monitor-pane', fragment);
  const image = $('.monitor-live-image', pane);
  const loading = $('.monitor-pane-loading', pane);
  const errorBox = $('.monitor-pane-error', pane);
  const remove = $('.monitor-remove', pane);

  pane.dataset.cameraId = camera.id;
  pane.classList.add('entering');
  $('.monitor-pane-name', pane).textContent = camera.name;
  image.alt = `Transmisión en vivo de ${camera.name}`;
  image.addEventListener('load', () => {
    loading.hidden = true;
    errorBox.hidden = true;
    pane.classList.remove('entering');
  });
  image.addEventListener('error', async () => {
    loading.hidden = true;
    errorBox.hidden = false;
    try {
      const session = await getSessionStatus();
      if (session.required && !session.authenticated) showLogin('Tu sesión terminó. Ingresa de nuevo para continuar.');
    } catch (_) {}
  });
  remove.addEventListener('click', () => {
    state.monitorSelected = state.monitorSelected.filter(id => id !== camera.id);
    if (window.location.pathname.startsWith('/camara/') && state.monitorSelected.length !== 1) {
      setBrowserRoute('/multivista', { replace: true });
    }
    renderMonitor();
  });

  const quality = lowBandwidth ? '&quality=low' : '';
  image.src = `/api/cameras/${encodeURIComponent(camera.id)}/live?t=${Date.now()}${quality}`;
  return pane;
}

function renderMonitor() {
  const selected = state.monitorSelected
    .map(id => state.cameras.find(camera => camera.id === id))
    .filter(Boolean)
    .slice(0, 9);
  state.monitorSelected = selected.map(camera => camera.id);

  monitorStage.className = `monitor-stage ${monitorLayoutClass(selected.length)}`;
  monitorEmpty.hidden = selected.length > 0;
  $('#monitorMeta').textContent = `${selected.length} ${selected.length === 1 ? 'cámara activa' : 'cámaras activas'}`;
  $('#monitorTitle').textContent = selected.length === 1 ? selected[0].name : 'Multivista';
  $('#singleViewBtn').disabled = selected.length <= 1;

  const wanted = new Set(selected.map(camera => camera.id));
  for (const pane of $$('.monitor-pane', monitorStage)) {
    if (wanted.has(pane.dataset.cameraId)) continue;
    $('.monitor-live-image', pane)?.removeAttribute('src');
    pane.remove();
  }

  const existing = new Map($$('.monitor-pane', monitorStage).map(pane => [pane.dataset.cameraId, pane]));
  const lowBandwidth = selected.length > 1;
  for (const camera of selected) {
    let pane = existing.get(camera.id);
    if (!pane) {
      pane = createMonitorPane(camera, lowBandwidth);
    } else {
      const name = $('.monitor-pane-name', pane);
      if (name) name.textContent = camera.name;
    }
    monitorStage.append(pane);
  }

  renderPicker();
  requestAnimationFrame(fitMonitorStage);
}

function renderPicker() {
  if (!pickerList) return;
  const query = pickerSearch.value.trim();
  const cameras = state.cameras.filter(camera => cameraMatches(camera, query));
  pickerList.replaceChildren();
  $('#pickerCount').textContent = `${state.monitorSelected.length} / 9 seleccionadas`;

  for (const camera of cameras) {
    const label = document.createElement('label');
    label.className = 'picker-item';
    const selected = state.monitorSelected.includes(camera.id);
    label.classList.toggle('selected', selected);
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = selected;
    checkbox.disabled = !selected && state.monitorSelected.length >= 9;
    const details = document.createElement('div');
    const name = document.createElement('strong');
    const meta = document.createElement('span');
    name.textContent = camera.name;
    meta.textContent = [camera.manufacturer, camera.model].filter(Boolean).join(' · ') || camera.endpoint;
    details.append(name, meta);
    label.append(checkbox, details);

    checkbox.addEventListener('change', () => {
      if (checkbox.checked) {
        if (state.monitorSelected.length >= 9) {
          checkbox.checked = false;
          setStatus('La multivista admite hasta 9 cámaras simultáneas.', false);
          return;
        }
        state.monitorSelected.push(camera.id);
      } else {
        state.monitorSelected = state.monitorSelected.filter(id => id !== camera.id);
      }
      if (window.location.pathname.startsWith('/camara/') && state.monitorSelected.length !== 1) {
        setBrowserRoute('/multivista', { replace: true });
      }
      renderMonitor();
    });
    pickerList.append(label);
  }
}

function formatBytes(bytes) {
  const value = Number(bytes) || 0;
  if (value < 1024) return `${value} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let size = value / 1024;
  let unit = units[0];
  for (let i = 1; i < units.length && size >= 1024; i++) {
    size /= 1024;
    unit = units[i];
  }
  return `${size >= 10 ? size.toFixed(1) : size.toFixed(2)} ${unit}`;
}

function formatDuration(seconds) {
  const total = Math.max(0, Math.round(Number(seconds) || 0));
  if (total < 60) return `${total} s`;
  if (total < 3600) return `${Math.floor(total / 60)} min`;
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  return `${hours} h${minutes ? ` ${minutes} min` : ''}`;
}

function formatClock(seconds) {
  const total = Math.max(0, Math.floor(Number(seconds) || 0));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  return hours > 0
    ? `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
    : `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

function localDateKey(value) {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

async function loadRecordings() {
  const loading = $('#recordingsLoading');
  const empty = $('#recordingsEmpty');
  const workspace = $('#recordingsWorkspace');
  renderRecordingSkeletons();
  loading.hidden = false;
  empty.hidden = true;
  $('h2', empty).textContent = 'No hay grabaciones almacenadas';
  $('p', empty).textContent = 'Activa la grabación por eventos en una cámara para que los clips aparezcan aquí.';
  workspace.hidden = true;
  try {
    state.recordings = await api('/api/recordings') || [];
    const cameraStillExists = state.cameras.some(camera => camera.id === state.recordingCameraID);
    if (!cameraStillExists) {
      const firstWithRecording = state.cameras.find(camera => state.recordings.some(recording => recording.camera_id === camera.id));
      state.recordingCameraID = firstWithRecording?.id || state.cameras[0]?.id || '';
    }
    renderRecordingCameraList();
    loading.hidden = true;
    if (!state.cameras.length) {
      empty.hidden = false;
      return;
    }
    workspace.hidden = false;
    const pending = state.pendingPlayback;
    if (pending) {
      state.recordingCameraID = pending.cameraID;
      selectRecordingCamera(pending.cameraID, false);
      const recording = state.recordings.find(item => item.camera_id === pending.cameraID && item.name === pending.recordingName);
      state.pendingPlayback = null;
      if (recording) selectRecording(recording, pending.offset);
    } else {
      selectRecordingCamera(state.recordingCameraID, Boolean(state.recordingDate));
    }
    setStatus('Biblioteca de grabaciones actualizada', true);
  } catch (error) {
    loading.hidden = true;
    if (error instanceof SessionRequiredError) return;
    empty.hidden = false;
    $('h2', empty).textContent = 'No se pudo cargar el almacenamiento';
    $('p', empty).textContent = error.message;
    setStatus(error.message, false);
  }
}

function recordingsForCamera(cameraID) {
  return state.recordings.filter(recording => recording.camera_id === cameraID);
}

async function loadRecordingAnalytics({ silent = false } = {}) {
  if (state.analyticsLoading || !state.recordingCameraID) return;
  state.analyticsLoading = true;
  try {
    const cameraID = encodeURIComponent(state.recordingCameraID);
    const [status, segments] = await Promise.all([
      api('/api/analytics/status'),
      api(`/api/analytics/persons?camera_id=${cameraID}`),
    ]);
    state.analyticsStatus = status;
    state.personSegments = segments || [];
    renderAnalyticsStatus();
    renderPersonTimeline(recordingsForCamera(state.recordingCameraID));
  } catch (error) {
    if (!(error instanceof SessionRequiredError) && !silent) setStatus(error.message, false);
  } finally {
    state.analyticsLoading = false;
  }
}

function renderAnalyticsStatus() {
  const element = $('#analyticsStatus');
  const button = $('#analyticsReindexBtn');
  const status = state.analyticsStatus;
  if (!status) {
    element.textContent = 'Analítica…';
    button.disabled = true;
    return;
  }
  button.disabled = !status.ready;
  if (!status.ready) {
    element.textContent = 'Analítica no disponible';
    element.title = status.last_error || 'NanoDet no está disponible.';
    element.classList.add('unavailable');
    return;
  }
  element.classList.remove('unavailable');
  if (status.analyzing) {
    const progress = Number(status.progress_seconds) || 0;
    const duration = Number(status.duration_seconds) || 0;
    const percent = duration > 0 ? Math.min(100, Math.round(progress / duration * 100)) : 0;
    element.textContent = `NanoDet · analizando ${percent}%`;
  } else {
    const count = state.personSegments.length;
    element.textContent = `NanoDet · 1 FPS · ${count} ${count === 1 ? 'tramo' : 'tramos'}`;
  }
  element.title = 'Detección local de personas con NanoDet-Plus-m 320.';
}

async function reindexAnalytics() {
  const button = $('#analyticsReindexBtn');
  button.disabled = true;
  try {
    await api('/api/analytics/reindex', { method: 'POST', body: '{}' });
    setStatus('Reanálisis de personas agregado a la cola', true);
    setTimeout(() => loadRecordingAnalytics({ silent: true }), 600);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    renderAnalyticsStatus();
  }
}

function renderRecordingCameraList() {
  const list = $('#recordingCameraList');
  list.replaceChildren();
  for (const camera of state.cameras) {
    const recordings = recordingsForCamera(camera.id);
    const size = recordings.reduce((sum, recording) => sum + (Number(recording.size_bytes) || 0), 0);
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'recording-camera-item';
    button.classList.toggle('active', camera.id === state.recordingCameraID);
    const details = document.createElement('div');
    const name = document.createElement('strong');
    const meta = document.createElement('span');
    const count = document.createElement('span');
    name.textContent = camera.name;
    meta.textContent = recordings.length ? `${formatBytes(size)} · ${recordings.length} clips` : 'Sin grabaciones';
    count.className = 'recording-camera-count';
    count.textContent = String(recordings.length);
    details.append(name, meta);
    button.append(details, count);
    button.addEventListener('click', () => selectRecordingCamera(camera.id, false, true));
    list.append(button);
  }
}

function selectRecordingCamera(cameraID, preserveDate = false, updateRoute = false) {
  if (!cameraID) return;
  stopRecordingPlayback();
  $('#recordingPlayer').hidden = true;
  const cameraChanged = state.recordingCameraID !== cameraID;
  state.recordingCameraID = cameraID;
  if (cameraChanged) state.personSegments = [];
  if (updateRoute) setBrowserRoute(`/grabaciones/${encodeURIComponent(cameraID)}`);
  const camera = state.cameras.find(item => item.id === cameraID);
  const recordings = recordingsForCamera(cameraID);
  if (!preserveDate || !state.recordingDate) {
    const activeRecording = recordings.find(recording => recording.active);
    state.recordingDate = activeRecording ? localDateKey(new Date()) : (recordings.length ? localDateKey(recordings[0].started_at) : localDateKey(new Date()));
  }
  $('#recordingDate').value = state.recordingDate;
  $('#recordingCameraTitle').textContent = camera?.name || 'Grabaciones';
  renderRecordingCameraList();
  renderRecordingStats(recordings);
  renderRecordingTimeline(recordings);
  renderPersonTimeline(recordings);
  renderRecordingCards(recordings);
  loadRecordingAnalytics({ silent: true });
}

function renderRecordingStats(recordings) {
  const size = recordings.reduce((sum, recording) => sum + (Number(recording.size_bytes) || 0), 0);
  const duration = recordings.reduce((sum, recording) => sum + (Number(recording.duration_seconds) || 0), 0);
  $('#recordingCount').textContent = String(recordings.length);
  $('#recordingSize').textContent = formatBytes(size);
  $('#recordingDuration').textContent = formatDuration(duration);
}

function selectedDateBounds() {
  const parts = String(state.recordingDate || '').split('-').map(Number);
  if (parts.length !== 3 || parts.some(value => !Number.isFinite(value))) return null;
  const start = new Date(parts[0], parts[1] - 1, parts[2], 0, 0, 0, 0);
  const end = new Date(parts[0], parts[1] - 1, parts[2] + 1, 0, 0, 0, 0);
  return { start: start.getTime(), end: end.getTime(), span: end.getTime() - start.getTime() };
}

function recordingInterval(recording) {
  const start = new Date(recording.started_at).getTime();
  if (!Number.isFinite(start)) return null;
  const duration = Math.max(1, Number(recording.duration_seconds) || 1) * 1000;
  return { start, end: start + duration };
}

function timelineGeometry(start, end, bounds) {
  const visibleStart = Math.max(start, bounds.start);
  const visibleEnd = Math.min(end, bounds.end);
  if (!(visibleEnd > visibleStart)) return null;
  const left = Math.max(0, Math.min(100, (visibleStart - bounds.start) / bounds.span * 100));
  const width = Math.max(.22, Math.min(100 - left, (visibleEnd - visibleStart) / bounds.span * 100));
  return { left, width };
}

function recordingsOnSelectedDate(recordings) {
  const bounds = selectedDateBounds();
  if (!bounds) return [];
  return recordings.filter(recording => {
    const interval = recordingInterval(recording);
    return interval && interval.end > bounds.start && interval.start < bounds.end;
  });
}

function renderRecordingTimeline(recordings) {
  const timeline = $('#recordingTimeline');
  const clips = recordingsOnSelectedDate(recordings);
  const bounds = selectedDateBounds();
  timeline.replaceChildren();
  $('#timelineEmpty').hidden = clips.length > 0;
  if (!bounds) return;

  for (const recording of clips) {
    const interval = recordingInterval(recording);
    const geometry = interval ? timelineGeometry(interval.start, interval.end, bounds) : null;
    if (!geometry) continue;
    const date = new Date(recording.started_at);
    const duration = Math.max(1, Number(recording.duration_seconds) || 1);
    const clip = document.createElement('button');
    clip.type = 'button';
    clip.className = 'timeline-clip';
    clip.classList.toggle('active', state.playback.recording?.name === recording.name && state.playback.recording?.camera_id === recording.camera_id);
    clip.style.left = `${geometry.left}%`;
    clip.style.width = `${geometry.width}%`;
    clip.title = `${date.toLocaleTimeString()} · ${formatDuration(duration)}`;
    clip.setAttribute('aria-label', `Reproducir grabación de ${date.toLocaleTimeString()}`);
    clip.addEventListener('click', () => selectRecording(recording));
    timeline.append(clip);
  }
}

function renderPersonTimeline(recordings) {
  const timeline = $('#personTimeline');
  if (!timeline) return;
  timeline.replaceChildren();
  const bounds = selectedDateBounds();
  if (!bounds) return;
  const byName = new Map(recordings.map(recording => [recording.name, recording]));

  for (const segment of state.personSegments) {
    const recording = byName.get(segment.recording_name);
    if (!recording) continue;
    const recordingStart = new Date(recording.started_at).getTime();
    if (!Number.isFinite(recordingStart)) continue;
    const start = recordingStart + Math.max(0, Number(segment.start_seconds) || 0) * 1000;
    const end = recordingStart + Math.max(Number(segment.end_seconds) || 0, Number(segment.start_seconds) || 0) * 1000;
    const geometry = timelineGeometry(start, Math.max(start + 1000, end), bounds);
    if (!geometry) continue;

    const marker = document.createElement('button');
    marker.type = 'button';
    marker.className = 'person-segment';
    marker.style.left = `${geometry.left}%`;
    marker.style.width = `${geometry.width}%`;
    const confidence = Math.round((Number(segment.max_confidence) || 0) * 100);
    const moment = new Date(recordingStart + (Number(segment.peak_seconds) || 0) * 1000);
    marker.title = `Persona · ${moment.toLocaleTimeString()} · ${confidence}%`;
    marker.setAttribute('aria-label', `Abrir detección de persona a las ${moment.toLocaleTimeString()}`);
    marker.addEventListener('click', () => selectRecording(recording, Number(segment.peak_seconds) || 0));
    timeline.append(marker);
  }
}

function renderRecordingCards(recordings) {
  const list = $('#recordingList');
  list.replaceChildren();
  $('#recordingListMeta').textContent = `${recordings.length} ${recordings.length === 1 ? 'grabación' : 'grabaciones'}`;

  if (!recordings.length) {
    const empty = document.createElement('div');
    empty.className = 'placeholder-panel';
    empty.innerHTML = '<div class="empty-icon" aria-hidden="true">◫</div><h2>Esta cámara todavía no tiene clips</h2><p>Las grabaciones aparecerán aquí cuando se active un evento compatible.</p>';
    list.append(empty);
    return;
  }

  for (const recording of recordings) {
    const card = document.createElement('article');
    card.className = 'recording-card';
    card.classList.toggle('active', state.playback.recording?.name === recording.name && state.playback.recording?.camera_id === recording.camera_id);
    const preview = document.createElement('button');
    preview.type = 'button';
    preview.className = 'recording-preview loading';
    const image = document.createElement('img');
    const fallback = document.createElement('div');
    const play = document.createElement('span');
    image.alt = `Miniatura de grabación ${new Date(recording.started_at).toLocaleString()}`;
    image.loading = 'lazy';
    image.src = recordingSnapshotURL(recording);
    fallback.className = 'recording-preview-fallback';
    fallback.hidden = true;
    fallback.innerHTML = '<span aria-hidden="true">◫</span><strong>Sin miniatura</strong>';
    play.className = 'recording-card-play';
    play.innerHTML = '<i aria-hidden="true">▶</i>';
    image.addEventListener('load', () => preview.classList.remove('loading'));
    image.addEventListener('error', () => { preview.classList.remove('loading'); image.hidden = true; fallback.hidden = false; });
    preview.append(image, fallback, play);
    if (recording.active) {
      const badge = document.createElement('span');
      badge.className = 'recording-live-badge';
      badge.textContent = 'GRABANDO';
      preview.append(badge);
    }
    preview.addEventListener('click', () => selectRecording(recording));

    const body = document.createElement('div');
    body.className = 'recording-card-body';
    const title = document.createElement('h3');
    title.className = 'recording-card-title';
    title.textContent = new Date(recording.started_at).toLocaleString();
    const meta = document.createElement('div');
    meta.className = 'recording-card-meta';
    const duration = document.createElement('span');
    const size = document.createElement('span');
    duration.textContent = formatDuration(recording.duration_seconds);
    size.textContent = formatBytes(recording.size_bytes);
    meta.append(duration, size);
    body.append(title, meta);
    card.append(preview, body);
    list.append(card);
  }
}

function recordingBaseURL(recording) {
  return `/api/recordings/${encodeURIComponent(recording.camera_id)}/${encodeURIComponent(recording.name)}`;
}

function recordingSnapshotURL(recording) {
  return `${recordingBaseURL(recording)}/snapshot`;
}

function selectRecording(recording, offset = 0) {
  const recordingStart = new Date(recording.started_at).getTime();
  const targetTime = Number.isFinite(recordingStart) ? new Date(recordingStart + Math.max(0, Number(offset) || 0) * 1000) : new Date(recording.started_at);
  state.recordingDate = localDateKey(targetTime);
  $('#recordingDate').value = state.recordingDate;
  $('#recordingPlayer').hidden = false;
  $('#recordingPlayerTitle').textContent = new Date(recording.started_at).toLocaleTimeString();
  $('#recordingPlayerDate').textContent = new Date(recording.started_at).toLocaleString();
  $('#recordingDownload').href = `${recordingBaseURL(recording)}/download`;
  $('#recordingScrubber').max = String(Math.max(1, Number(recording.duration_seconds) || 1));
  $('#recordingTotalTime').textContent = formatClock(recording.duration_seconds);
  startRecordingPlayback(recording, offset);
  renderRecordingTimeline(recordingsForCamera(recording.camera_id));
  renderPersonTimeline(recordingsForCamera(recording.camera_id));
  renderRecordingCards(recordingsForCamera(recording.camera_id));
  $('#recordingPlayer').scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function startRecordingPlayback(recording, offset = 0) {
  stopRecordingPlayback(false);
  const duration = Math.max(0, Number(recording.duration_seconds) || 0);
  const safeOffset = Math.max(0, duration > 0 ? Math.min(offset, Math.max(0, duration - .1)) : offset);
  state.playback.recording = recording;
  state.playback.offset = safeOffset;
  state.playback.startedAt = performance.now();
  state.playback.playing = true;

  const image = $('#recordingPlaybackImage');
  const loading = $('#recordingPlaybackLoading');
  const errorBox = $('#recordingPlaybackError');
  loading.hidden = false;
  errorBox.hidden = true;
  image.hidden = false;
  image.onload = () => { loading.hidden = true; errorBox.hidden = true; };
  image.onerror = async () => {
    loading.hidden = true;
    if (state.playback.playing) errorBox.hidden = false;
    try {
      const session = await getSessionStatus();
      if (session.required && !session.authenticated) showLogin('Tu sesión terminó. Ingresa de nuevo para continuar.');
    } catch (_) {}
  };
  image.src = `${recordingBaseURL(recording)}/play?start=${safeOffset.toFixed(3)}&t=${Date.now()}`;
  $('#recordingPlayPause').textContent = 'Pausar';
  updatePlaybackUI(safeOffset);
  state.playback.timer = setInterval(updatePlaybackProgress, 400);
}

function currentPlaybackOffset() {
  if (!state.playback.recording) return 0;
  if (!state.playback.playing) return state.playback.offset;
  return state.playback.offset + (performance.now() - state.playback.startedAt) / 1000;
}

function updatePlaybackProgress() {
  const recording = state.playback.recording;
  if (!recording || !state.playback.playing) return;
  const duration = Math.max(0, Number(recording.duration_seconds) || 0);
  const current = currentPlaybackOffset();
  if (duration > 0 && current >= duration) {
    state.playback.offset = duration;
    state.playback.playing = false;
    $('#recordingPlaybackImage').removeAttribute('src');
    if (state.playback.timer !== null) clearInterval(state.playback.timer);
    state.playback.timer = null;
    $('#recordingPlayPause').textContent = 'Reproducir';
    updatePlaybackUI(duration);
    return;
  }
  updatePlaybackUI(current);
}

function updatePlaybackUI(seconds) {
  const scrubber = $('#recordingScrubber');
  const max = Number(scrubber.max) || 1;
  const value = Math.max(0, Math.min(max, Number(seconds) || 0));
  scrubber.value = String(value);
  $('#recordingCurrentTime').textContent = formatClock(value);
}

function stopRecordingPlayback(clearRecording = true) {
  if (state.playback.playing) state.playback.offset = currentPlaybackOffset();
  state.playback.playing = false;
  if (state.playback.timer !== null) clearInterval(state.playback.timer);
  state.playback.timer = null;
  const image = $('#recordingPlaybackImage');
  if (image) image.removeAttribute('src');
  if (clearRecording) {
    state.playback.recording = null;
    state.playback.offset = 0;
  }
}

function toggleRecordingPlayback() {
  const recording = state.playback.recording;
  if (!recording) return;
  if (state.playback.playing) {
    state.playback.offset = currentPlaybackOffset();
    state.playback.playing = false;
    if (state.playback.timer !== null) clearInterval(state.playback.timer);
    state.playback.timer = null;
    $('#recordingPlaybackImage').removeAttribute('src');
    $('#recordingPlayPause').textContent = 'Reproducir';
    updatePlaybackUI(state.playback.offset);
    return;
  }
  startRecordingPlayback(recording, state.playback.offset);
}

async function refreshEvents(updateStatus = true) {
  const list = $('#eventsList');
  const shouldSkeleton = state.currentView === 'events' && !list.children.length;
  if (shouldSkeleton) renderEventSkeletons();
  try {
    const events = await api('/api/events');
    renderEvents(events || []);
    if (updateStatus) setStatus('Eventos actualizados', true);
  } catch (error) {
    if (error instanceof SessionRequiredError) return;
    if (shouldSkeleton) renderLoadError(list, 'No se pudieron cargar los eventos', error.message);
    if (updateStatus) setStatus(error.message, false);
  }
}

function eventKey(event) {
  return [event.camera_id, event.time, event.topic || '', event.data?.State || ''].join('|');
}

function eventRecordingSnapshotURL(event, attempt = 0) {
  if (!event.recording_name) return '';
  const offset = Math.max(0, Number(event.recording_offset) || 0);
  return `/api/recordings/${encodeURIComponent(event.camera_id)}/${encodeURIComponent(event.recording_name)}/snapshot?at=${offset.toFixed(3)}&r=${attempt}`;
}

function eventCameraSnapshotURL(event) {
  return `/api/cameras/${encodeURIComponent(event.camera_id)}/snapshot?v=${encodeURIComponent(event.time || Date.now())}`;
}

function loadEventThumbnail(image, event) {
  let attempt = 0;
  const finishLoading = () => image.closest('.event-thumb')?.classList.remove('loading');
  const fallback = () => {
    image.onerror = finishLoading;
    image.src = eventCameraSnapshotURL(event);
  };
  if (!event.recording_name) {
    fallback();
    return;
  }
  const load = () => {
    image.src = eventRecordingSnapshotURL(event, attempt);
  };
  image.onerror = () => {
    if (attempt < 3 && image.isConnected) {
      attempt += 1;
      setTimeout(load, 1400);
      return;
    }
    fallback();
  };
  load();
}

function openEventRecording(event) {
  if (!event.recording_name) return;
  state.pendingPlayback = {
    cameraID: event.camera_id,
    recordingName: event.recording_name,
    offset: Math.max(0, Number(event.recording_offset) || 0),
  };
  state.recordingCameraID = event.camera_id;
  navigate('recordings', { path: `/grabaciones/${encodeURIComponent(event.camera_id)}` });
}

function createEventCard(event) {
  const card = document.createElement(event.recording_name ? 'button' : 'article');
  if (card instanceof HTMLButtonElement) card.type = 'button';
  card.className = 'event-card event-enter';
  card.dataset.eventKey = eventKey(event);
  if (event.recording_name) {
    card.classList.add('clickable');
    card.addEventListener('click', () => openEventRecording(event));
  }

  const thumb = document.createElement('div');
  thumb.className = 'event-thumb loading';
  const image = document.createElement('img');
  image.alt = `Evento de ${event.camera_name}`;
  image.loading = 'lazy';
  image.addEventListener('load', () => thumb.classList.remove('loading'));
  thumb.append(image);
  loadEventThumbnail(image, event);

  const body = document.createElement('div');
  body.className = 'event-body';
  const camera = document.createElement('strong');
  camera.textContent = event.camera_name;
  const topic = document.createElement('span');
  topic.className = 'event-topic';
  topic.textContent = (event.topic || 'Evento').split('/').pop() || 'Evento';
  const details = document.createElement('span');
  details.className = 'event-detail';
  const score = event.data?.Score ? ` · nivel ${Math.round(Number(event.data.Score) * 100)}%` : '';
  details.textContent = `${new Date(event.time).toLocaleString()}${score}`;
  body.append(camera, topic, details);

  const side = document.createElement('div');
  side.className = 'event-side';
  if (event.recording_name) {
    const action = document.createElement('span');
    action.className = 'event-jump';
    action.textContent = `▶ ${formatClock(event.recording_offset || 0)}`;
    side.append(action);
  }
  if (event.recording_trigger) {
    const trigger = document.createElement('span');
    trigger.className = 'event-trigger';
    trigger.textContent = 'GRABADO';
    side.append(trigger);
  }

  card.append(thumb, body, side);
  requestAnimationFrame(() => card.classList.remove('event-enter'));
  return card;
}

function renderEvents(events) {
  const list = $('#eventsList');
  const visible = (events || []).slice(0, 100);
  if (!visible.length) {
    if (!$('.placeholder-panel', list)) {
      list.replaceChildren();
      const empty = document.createElement('div');
      empty.className = 'placeholder-panel';
      empty.innerHTML = '<div class="empty-icon" aria-hidden="true">◉</div><h2>Sin eventos recientes</h2><p>El visor mostrará aquí el movimiento detectado por ONVIF o por el detector de respaldo.</p>';
      list.append(empty);
    }
    return;
  }

  $('.placeholder-panel', list)?.remove();
  const existing = new Map($$('.event-card', list).map(card => [card.dataset.eventKey, card]));
  const ordered = visible.map(event => existing.get(eventKey(event)) || createEventCard(event));
  const wanted = new Set(ordered);
  for (const card of $$('.event-card', list)) {
    if (!wanted.has(card)) card.remove();
  }
  ordered.forEach((card, index) => {
    const current = list.children[index];
    if (current !== card) list.insertBefore(card, current || null);
  });
}

function describeSessionDevice(userAgent) {
  const ua = String(userAgent || '');
  let browser = 'Navegador';
  if (/Edg\//i.test(ua)) browser = 'Edge';
  else if (/Firefox\//i.test(ua)) browser = 'Firefox';
  else if (/Chrome\//i.test(ua)) browser = 'Chrome';
  else if (/Safari\//i.test(ua)) browser = 'Safari';
  let device = 'Equipo';
  if (/Android/i.test(ua)) device = 'Android';
  else if (/iPhone|iPad/i.test(ua)) device = 'iPhone/iPad';
  else if (/Windows/i.test(ua)) device = 'Windows';
  else if (/Macintosh/i.test(ua)) device = 'Mac';
  else if (/Linux/i.test(ua)) device = 'Linux';
  return `${browser} · ${device}`;
}

async function loadSessions() {
  const list = $('#sessionsList');
  renderSessionSkeletons();
  $('#closeOtherSessionsBtn').disabled = true;
  try {
    state.sessions = await api('/api/sessions') || [];
    renderSessions();
  } catch (error) {
    if (error instanceof SessionRequiredError) return;
    list.innerHTML = '';
    const errorBox = document.createElement('div');
    errorBox.className = 'placeholder-panel';
    errorBox.innerHTML = '<h2>No se pudieron cargar las sesiones</h2>';
    const copy = document.createElement('p');
    copy.textContent = error.message;
    errorBox.append(copy);
    list.append(errorBox);
  }
}

function renderSessions() {
  const list = $('#sessionsList');
  list.replaceChildren();
  const others = state.sessions.filter(session => !session.current);
  $('#closeOtherSessionsBtn').disabled = others.length === 0;
  if (!state.sessions.length) {
    list.innerHTML = '<div class="placeholder-panel"><h2>Sin sesiones activas</h2><p>Vuelve a iniciar sesión.</p></div>';
    return;
  }
  for (const session of state.sessions) {
    const card = document.createElement('article');
    card.className = `session-card${session.current ? ' current' : ''}`;
    const icon = document.createElement('span');
    icon.className = 'session-device-icon';
    icon.textContent = /Android|iPhone|iPad/i.test(session.user_agent || '') ? '▯' : '▰';
    const details = document.createElement('div');
    details.className = 'session-details';
    const title = document.createElement('div');
    title.className = 'session-title';
    const strong = document.createElement('strong');
    strong.textContent = describeSessionDevice(session.user_agent);
    title.append(strong);
    if (session.current) {
      const badge = document.createElement('span');
      badge.className = 'status-badge';
      badge.textContent = 'ACTUAL';
      title.append(badge);
    }
    const meta = document.createElement('span');
    meta.textContent = `${session.ip_address || 'IP local'} · actividad ${new Date(session.last_seen_at).toLocaleString()}`;
    const expires = document.createElement('span');
    expires.className = 'session-expiry';
    expires.textContent = `Expira ${new Date(session.expires_at).toLocaleString()}`;
    details.append(title, meta, expires);
    card.append(icon, details);
    if (!session.current) {
      const revoke = document.createElement('button');
      revoke.type = 'button';
      revoke.className = 'button ghost session-revoke';
      revoke.textContent = 'Cerrar';
      revoke.addEventListener('click', async () => {
        revoke.disabled = true;
        try {
          await api(`/api/sessions/${encodeURIComponent(session.id)}`, { method: 'DELETE' });
          await loadSessions();
        } catch (error) {
          if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
          revoke.disabled = false;
        }
      });
      card.append(revoke);
    }
    list.append(card);
  }
}

async function closeOtherSessions() {
  const others = state.sessions.filter(session => !session.current);
  if (!others.length) return;
  const accepted = await confirmAction({
    title: 'Cerrar las demás sesiones',
    message: `Se cerrarán ${others.length} sesión(es) en otros dispositivos. Esta sesión permanecerá activa.`,
    confirmLabel: 'Cerrar las demás',
  });
  if (!accepted) return;
  const button = $('#closeOtherSessionsBtn');
  button.disabled = true;
  try {
    const result = await api('/api/sessions/others', { method: 'DELETE' });
    setStatus(`${result?.deleted || 0} sesión(es) cerradas`, true);
    await loadSessions();
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    button.disabled = false;
  }
}

async function loadProfile() {
  const loading = $('#profileLoading');
  const form = $('#profileForm');
  const errorBox = $('#profileError');
  const messageBox = $('#profileMessage');
  loading.hidden = false;
  form.hidden = true;
  errorBox.textContent = '';
  messageBox.textContent = '';
  try {
    state.profile = await api('/api/profile');
    const username = state.profile?.username || '';
    $('#profileUsername').value = username;
    $('#profileDisplayName').textContent = username || 'Usuario';
    $('#profileAvatar').textContent = (username.trim()[0] || 'U').toUpperCase();
    $('#profileCurrentPassword').value = '';
    $('#profileNewPassword').value = '';
    $('#profileConfirmPassword').value = '';
    form.hidden = false;
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) {
      errorBox.textContent = error.message;
      form.hidden = false;
    }
  } finally {
    loading.hidden = true;
  }
}

async function saveProfile(event) {
  event.preventDefault();
  const username = $('#profileUsername').value.trim();
  const currentPassword = $('#profileCurrentPassword').value;
  const newPassword = $('#profileNewPassword').value;
  const confirmPassword = $('#profileConfirmPassword').value;
  const errorBox = $('#profileError');
  const messageBox = $('#profileMessage');
  const button = $('#profileSaveBtn');
  errorBox.textContent = '';
  messageBox.textContent = '';

  if (!username || !currentPassword) {
    errorBox.textContent = 'Escribe el usuario y tu contraseña actual.';
    return;
  }
  if (newPassword && newPassword.length < 8) {
    errorBox.textContent = 'La contraseña nueva debe tener al menos 8 caracteres.';
    return;
  }
  if (newPassword !== confirmPassword) {
    errorBox.textContent = 'Las contraseñas nuevas no coinciden.';
    return;
  }

  button.disabled = true;
  const previousLabel = button.textContent;
  button.textContent = 'Guardando…';
  try {
    const result = await api('/api/profile', {
      method: 'PUT',
      body: JSON.stringify({
        username,
        current_password: currentPassword,
        new_password: newPassword,
      }),
    });
    state.profile = { username: result.username };
    state.auth.username = result.username;
    $('#profileDisplayName').textContent = result.username;
    $('#profileAvatar').textContent = (result.username?.trim()[0] || 'U').toUpperCase();
    $('#profileCurrentPassword').value = '';
    $('#profileNewPassword').value = '';
    $('#profileConfirmPassword').value = '';
    const closed = Number(result.other_sessions_closed) || 0;
    messageBox.textContent = result.password_changed
      ? `Perfil actualizado. ${closed} sesión(es) adicional(es) cerradas.`
      : 'Perfil actualizado correctamente.';
    setStatus('Perfil actualizado', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) errorBox.textContent = error.message;
  } finally {
    button.disabled = false;
    button.textContent = previousLabel;
  }
}

async function loadTelegramStatus({ skeleton = state.telegram === null } = {}) {
  const loading = $('#telegramLoading');
  if (skeleton) {
    $('#telegramCardStatus').textContent = 'Consultando…';
    $('#telegramCardStatus').classList.add('skeleton-text');
    if (!$('#telegramPanel').hidden) {
      loading.hidden = false;
      $('#telegramTokenForm').hidden = true;
      $('#telegramConfigured').hidden = true;
    }
  }
  try {
    state.telegram = await api('/api/integrations/telegram');
    renderTelegramStatus();
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) {
      $('#telegramCardStatus').textContent = 'No disponible';
      $('#telegramSummary').textContent = error.message;
      $('#telegramTokenForm').hidden = true;
      $('#telegramConfigured').hidden = true;
      setStatus(error.message, false);
    }
  } finally {
    loading.hidden = true;
    $('#telegramCardStatus').classList.remove('skeleton-text');
  }
}

function renderTelegramStatus() {
  const data = state.telegram || { configured: false, media_mode: 'photo', video_seconds: 5, chats: [] };
  $('#telegramCardStatus').textContent = data.configured ? `@${data.bot_username}` : 'Sin configurar';
  $('#telegramTokenForm').hidden = Boolean(data.configured);
  $('#telegramConfigured').hidden = !data.configured;
  $('#telegramSummary').textContent = data.configured
    ? `Bot @${data.bot_username} conectado.`
    : 'Conecta un bot para alertas y consultas remotas.';
  if (!data.configured) return;
  $('#telegramBotName').textContent = `@${data.bot_username}`;
  $('#telegramBotMeta').textContent = `ID ${data.bot_id}`;
  $('#telegramBotLink').href = data.bot_url || '#';
  $('#telegramNotificationsEnabled').checked = Boolean(data.notifications_enabled);
  $('#telegramMediaMode').value = data.media_mode || 'photo';
  $('#telegramVideoSeconds').value = String(data.video_seconds || 5);
  syncTelegramMediaFields();
  renderTelegramChats(data.chats || []);
}

function renderTelegramChats(chats) {
  const list = $('#telegramChats');
  list.replaceChildren();
  if (!chats.length) {
    const empty = document.createElement('p');
    empty.className = 'hint';
    empty.textContent = 'Todavía no hay chats vinculados.';
    list.append(empty);
    return;
  }
  for (const chat of chats) {
    const item = document.createElement('div');
    item.className = 'telegram-chat-item';
    const details = document.createElement('div');
    const strong = document.createElement('strong');
    const meta = document.createElement('span');
    strong.textContent = chat.chat_name || `Chat ${chat.chat_id}`;
    meta.textContent = `Vinculado ${new Date(chat.linked_at).toLocaleString()}`;
    details.append(strong, meta);
    const badge = document.createElement('span');
    badge.className = 'status-badge';
    badge.textContent = chat.enabled ? 'ACTIVO' : 'PAUSADO';
    item.append(details, badge);
    list.append(item);
  }
}

function syncTelegramMediaFields() {
  $('#telegramVideoSeconds').disabled = $('#telegramMediaMode').value !== 'video';
}

async function saveTelegramToken(event) {
  event.preventDefault();
  const token = $('#telegramToken').value.trim();
  const errorBox = $('#telegramTokenError');
  const button = $('#telegramSaveTokenBtn');
  errorBox.textContent = '';
  if (!token) {
    errorBox.textContent = 'Escribe el token entregado por BotFather.';
    return;
  }
  button.disabled = true;
  button.textContent = 'Validando…';
  try {
    state.telegram = await api('/api/integrations/telegram', {
      method: 'PUT',
      body: JSON.stringify({ token, notifications_enabled: true, media_mode: 'photo', video_seconds: 5 }),
    });
    $('#telegramToken').value = '';
    renderTelegramStatus();
    setStatus('Bot de Telegram conectado', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) errorBox.textContent = error.message;
  } finally {
    button.disabled = false;
    button.textContent = 'Validar y guardar';
  }
}

async function saveTelegramSettings(event) {
  event.preventDefault();
  const button = $('#telegramSaveSettingsBtn');
  const videoSeconds = Number($('#telegramVideoSeconds').value) || 5;
  if (videoSeconds < 3 || videoSeconds > 60) {
    setStatus('La duración de Telegram debe estar entre 3 y 60 segundos.', false);
    return;
  }
  button.disabled = true;
  try {
    state.telegram = await api('/api/integrations/telegram', {
      method: 'PUT',
      body: JSON.stringify({
        token: '',
        notifications_enabled: $('#telegramNotificationsEnabled').checked,
        media_mode: $('#telegramMediaMode').value,
        video_seconds: videoSeconds,
      }),
    });
    renderTelegramStatus();
    setStatus('Opciones de Telegram guardadas', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    button.disabled = false;
  }
}

async function generateTelegramCode() {
  const button = $('#telegramGenerateCodeBtn');
  button.disabled = true;
  try {
    const result = await api('/api/integrations/telegram/code', { method: 'POST', body: '{}' });
    $('#telegramCodeValue').textContent = result.code;
    $('#telegramCodeExpiry').textContent = `Válido hasta ${new Date(result.expires_at).toLocaleTimeString()}`;
    $('#telegramDeepLink').href = result.deep_link;
    $('#telegramCodeCommands').textContent = (result.commands || []).join('   ·   ');
    $('#telegramCodeBox').hidden = false;
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    button.disabled = false;
  }
}

async function deleteTelegramIntegration() {
  const accepted = await confirmAction({
    title: 'Eliminar Telegram',
    message: 'Se eliminarán el token cifrado, los códigos temporales y todos los chats vinculados.',
    confirmLabel: 'Eliminar integración',
  });
  if (!accepted) return;
  const button = $('#telegramDeleteBtn');
  button.disabled = true;
  try {
    await api('/api/integrations/telegram', { method: 'DELETE' });
    state.telegram = { configured: false, media_mode: 'photo', video_seconds: 5, chats: [] };
    $('#telegramCodeBox').hidden = true;
    renderTelegramStatus();
    setStatus('Integración Telegram eliminada', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    button.disabled = false;
  }
}

async function discover() {
  const button = $('#discoverBtn');
  const status = $('#discoveryStatus');
  const list = $('#discoveryList');
  button.disabled = true;
  renderDiscoverySkeletons();
  status.textContent = 'Buscando cámaras ONVIF en la red local…';
  discoveryDialog.showModal();
  try {
    const devices = await api('/api/discover', { method: 'POST', body: '{}' });
    list.replaceChildren();
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
      add.className = 'button primary';
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
    list.replaceChildren();
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
  return new Promise(resolve => {
    confirmDialog.addEventListener('close', () => resolve(confirmDialog.returnValue === 'confirm'), { once: true });
  });
}

$('#scheduleEnabled').addEventListener('change', syncScheduleFormState);

$('#scheduleForm').addEventListener('submit', async (event) => {
  event.preventDefault();
  const cameraID = $('#scheduleCameraID').value;
  const enabled = $('#scheduleEnabled').checked;
  const days = $$('#scheduleDays input[type="checkbox"]:checked').map(input => Number(input.value));
  const start = $('#scheduleStart').value;
  const end = $('#scheduleEnd').value;
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'Local';
  const errorBox = $('#scheduleFormError');
  const saveButton = $('#saveScheduleBtn');
  errorBox.textContent = '';

  if (enabled && !days.length) {
    errorBox.textContent = 'Selecciona al menos un día.';
    return;
  }
  if (enabled && (!start || !end)) {
    errorBox.textContent = 'Selecciona la hora de inicio y la hora final.';
    return;
  }

  saveButton.disabled = true;
  try {
    const updated = await api(`/api/cameras/${encodeURIComponent(cameraID)}/schedule`, {
      method: 'PUT',
      body: JSON.stringify({ enabled, days, start, end, timezone }),
    });
    const camera = state.cameras.find(item => item.id === cameraID);
    if (camera && updated) Object.assign(camera, updated);
    scheduleDialog.close();
    renderCameras();
    setStatus(enabled ? 'Horario de grabación activado' : 'Horario de grabación desactivado', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) errorBox.textContent = error.message;
  } finally {
    saveButton.disabled = false;
  }
});

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
    state.snapshotRevision = Date.now();
    await refreshAll();
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) errorBox.textContent = error.message;
  } finally {
    saveButton.disabled = false;
  }
});

for (const item of $$('.nav-item[data-view]')) item.addEventListener('click', () => navigate(item.dataset.view));
$('.sidebar-brand').addEventListener('click', () => navigate('cameras'));
$('#addBtn').addEventListener('click', () => cameraDialog.showModal());
$('#emptyAddBtn').addEventListener('click', () => cameraDialog.showModal());
$('#discoverBtn').addEventListener('click', discover);
$('#emptyDiscoverBtn').addEventListener('click', discover);
$('#eventsRefreshBtn').addEventListener('click', () => refreshEvents(true));
$('#closeOtherSessionsBtn').addEventListener('click', closeOtherSessions);
$('#profileForm').addEventListener('submit', saveProfile);
$('#telegramOpenBtn').addEventListener('click', async () => {
  $('#telegramPanel').hidden = false;
  await loadTelegramStatus();
  $('#telegramPanel').scrollIntoView({ behavior: 'smooth', block: 'start' });
});
$('#telegramPanelCloseBtn').addEventListener('click', () => { $('#telegramPanel').hidden = true; });
$('#telegramTokenForm').addEventListener('submit', saveTelegramToken);
$('#telegramSettingsForm').addEventListener('submit', saveTelegramSettings);
$('#telegramGenerateCodeBtn').addEventListener('click', generateTelegramCode);
$('#telegramDeleteBtn').addEventListener('click', deleteTelegramIntegration);
$('#telegramMediaMode').addEventListener('change', syncTelegramMediaFields);
$('#monitorFullscreenBtn').addEventListener('click', toggleMonitorFullscreen);
$('#backToCamerasBtn').addEventListener('click', () => navigate('cameras'));
$('#singleViewBtn').addEventListener('click', () => {
  if (state.monitorSelected.length > 1) state.monitorSelected = [state.monitorSelected[0]];
  const cameraID = state.monitorSelected[0];
  if (cameraID) navigate('monitor', { path: `/camara/${encodeURIComponent(cameraID)}`, replace: true });
});
$('#togglePickerBtn').addEventListener('click', () => monitorPicker.classList.toggle('open'));
$('#closePickerBtn').addEventListener('click', () => monitorPicker.classList.remove('open'));
$('#monitorEmptyAddBtn').addEventListener('click', () => monitorPicker.classList.add('open'));
$('#refreshSnapshotsBtn').addEventListener('click', () => {
  state.snapshotRevision = Date.now();
  renderCameras();
  setStatus('Miniaturas actualizadas', true);
});
cameraSearch.addEventListener('input', renderCameras);
pickerSearch.addEventListener('input', renderPicker);
$('#recordingsRefreshBtn').addEventListener('click', loadRecordings);
$('#analyticsReindexBtn').addEventListener('click', reindexAnalytics);
$('#recordingDate').addEventListener('change', (event) => {
  state.recordingDate = event.target.value;
  const recordings = recordingsForCamera(state.recordingCameraID);
  renderRecordingTimeline(recordings);
  renderPersonTimeline(recordings);
});
$('#recordingPlayPause').addEventListener('click', toggleRecordingPlayback);
$('#recordingScrubber').addEventListener('input', (event) => {
  $('#recordingCurrentTime').textContent = formatClock(Number(event.target.value));
});
$('#recordingScrubber').addEventListener('change', (event) => {
  const recording = state.playback.recording;
  if (!recording) return;
  const offset = Number(event.target.value) || 0;
  const wasPlaying = state.playback.playing;
  if (wasPlaying) {
    startRecordingPlayback(recording, offset);
  } else {
    state.playback.offset = offset;
    updatePlaybackUI(offset);
  }
});
$('#recordingClosePlayer').addEventListener('click', () => {
  stopRecordingPlayback();
  $('#recordingPlayer').hidden = true;
  renderRecordingTimeline(recordingsForCamera(state.recordingCameraID));
  renderRecordingCards(recordingsForCamera(state.recordingCameraID));
});

for (const button of $$('[data-close]')) {
  button.addEventListener('click', () => {
    const dialog = document.getElementById(button.dataset.close);
    if (dialog?.open) dialog.close('cancel');
  });
}

window.addEventListener('popstate', () => {
  if (!appView.hidden) applyCurrentRoute();
});

window.addEventListener('resize', () => {
  if (!appView.hidden && state.currentView === 'monitor') fitMonitorStage();
});

document.addEventListener('fullscreenchange', syncFullscreenUI);
if ('ResizeObserver' in window) {
  const monitorResizeObserver = new ResizeObserver(() => {
    if (!appView.hidden && state.currentView === 'monitor') fitMonitorStage();
  });
  if (monitorStage.parentElement) monitorResizeObserver.observe(monitorStage.parentElement);
}

window.addEventListener('beforeunload', () => {
  stopMonitorStreams();
  stopRecordingPlayback();
});

bootstrap();
