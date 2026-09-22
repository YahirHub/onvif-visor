const state = {
  cameras: [],
  recordings: [],
  personSegments: [],
  objectSegments: [],
  analyticsClasses: [],
  analyticsConfig: null,
  analyticsCameraID: '',
  analyticsDrawing: null,
  analyticsRuleEditIndex: -1,
  analyticsSummary: null,
  analyticsSummaryKey: '',
  analyticsSummaryFetchedAt: 0,
  analyticsAnalyses: [],
  analyticsStatus: null,
  analyticsLoading: false,
  auth: { required: false, authenticated: false },
  currentView: 'cameras',
  monitorSelected: [],
  recordingCameraID: '',
  recordingDate: '',
  recordingName: '',
  recordingSummaries: [],
  recordingStorage: { total_bytes: 0, file_count: 0 },
  recordingPage: { items: [], page: 1, page_size: 24, total: 0, total_pages: 0 },
  playbackRouteOffset: 0,
  timelineZoom: 1,
  snapshotRevision: Date.now(),
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
  playback: { eyebrow: 'ARCHIVO', title: 'Reproducción', subtitle: 'Reproductor de grabación.' },
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
  const current = window.location.pathname + window.location.search;
  if (!path || current === path) return;
  window.history[replace ? 'replaceState' : 'pushState']({}, '', path);
}

function decodeRoutePart(value) {
  try { return decodeURIComponent(value); } catch (_) { return ''; }
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
    if (cameraID && !cameraID.includes('/')) return { view: 'monitor', cameraID: decodeRoutePart(cameraID) };
  }
  if (path.startsWith('/grabaciones/')) {
    const parts = path.slice('/grabaciones/'.length).split('/').filter(Boolean);
    if (parts.length === 2) {
      const recordingCameraID = decodeRoutePart(parts[0]);
      const recordingName = decodeRoutePart(parts[1]);
      const at = Number(new URLSearchParams(window.location.search).get('at')) || 0;
      if (recordingCameraID && recordingName) {
        return { view: 'playback', recordingCameraID, recordingName, playbackOffset: Math.max(0, at) };
      }
    }
    if (parts.length === 1) {
      const recordingCameraID = decodeRoutePart(parts[0]);
      if (recordingCameraID) return { view: 'recordings', recordingCameraID };
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
  if (route.clearRecordingCamera) {
    state.recordingCameraID = '';
    state.recordingName = '';
  }
  if (route.recordingCameraID) {
    if (!state.cameras.some(camera => camera.id === route.recordingCameraID)) {
      navigate('recordings', { replace: true });
      return;
    }
    state.recordingCameraID = route.recordingCameraID;
  }
  if (route.view === 'playback') {
    state.recordingName = route.recordingName;
    state.playbackRouteOffset = Math.max(0, Number(route.playbackOffset) || 0);
  } else if (route.view === 'recordings') {
    state.recordingName = '';
    state.playbackRouteOffset = 0;
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
  if (state.currentView === 'playback' && view !== 'playback') stopRecordingPlayback();
  if (view !== 'monitor') {
    monitorPicker.classList.remove('open');
    appView.classList.remove('theater-mode');
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  }
  state.currentView = view;
  appView.dataset.view = view;

  const activeNavView = view === 'playback' ? 'recordings' : view;
  for (const item of $$('.nav-item[data-view]')) item.classList.toggle('active', item.dataset.view === activeNavView);
  for (const section of $$('.app-view')) section.hidden = section.id !== `${view}View`;

  const meta = viewMeta[view];
  viewEyebrow.textContent = meta.eyebrow;
  viewTitle.textContent = meta.title;
  viewSubtitle.textContent = meta.subtitle;
  document.title = `${meta.title} · ONVIF Visor`;
  headerActions.hidden = view !== 'cameras';

  if (!fromHistory) {
    const targetPath = path || routePathByView[view];
    if (targetPath) setBrowserRoute(targetPath, { replace });
  }

  if (view === 'cameras') renderCameras();
  if (view === 'monitor') {
    if (!state.monitorSelected.length && state.cameras.length) state.monitorSelected = [state.cameras[0].id];
    renderMonitor();
  }
  if (view === 'recordings') loadRecordings();
  if (view === 'playback') loadDedicatedPlayback();
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
    if (cameraFingerprint(cameras) === cameraFingerprint(state.cameras)) return;

    const previousSelection = state.monitorSelected.join('|');
    state.cameras = cameras;
    state.monitorSelected = state.monitorSelected.filter(id => cameras.some(camera => camera.id === id));
    const selectionChanged = previousSelection !== state.monitorSelected.join('|');

    if (state.currentView === 'cameras') renderCameras();
    renderPicker();
    if (state.currentView === 'monitor' && selectionChanged) renderMonitor();
    if (state.currentView === 'recordings') renderRecordingCameraList();
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
    const analyticsButton = $('.camera-analytics', card);
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

    analyticsButton.addEventListener('click', () => openAnalyticsDialog(camera));
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

function recordingSummary(cameraID) {
  return state.recordingSummaries.find(item => item.camera_id === cameraID) || null;
}

function recordingDateRange() {
  const bounds = selectedDateBounds();
  if (!bounds) return null;
  return {
    from: new Date(bounds.start).toISOString(),
    to: new Date(bounds.end).toISOString(),
  };
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
    const summary = await api('/api/recordings/summary');
    state.recordingSummaries = summary?.cameras || [];
    state.recordingStorage = {
      total_bytes: Math.max(0, Number(summary?.total_bytes) || 0),
      file_count: Math.max(0, Number(summary?.file_count) || 0),
    };

    const cameraStillExists = state.cameras.some(camera => camera.id === state.recordingCameraID);
    if (!cameraStillExists) {
      const firstWithRecording = state.cameras.find(camera => (recordingSummary(camera.id)?.count || 0) > 0);
      state.recordingCameraID = firstWithRecording?.id || state.cameras[0]?.id || '';
      state.recordingDate = '';
    }

    renderRecordingCameraList();
    loading.hidden = true;
    if (!state.cameras.length) {
      empty.hidden = false;
      return;
    }

    workspace.hidden = false;
    await selectRecordingCamera(state.recordingCameraID, Boolean(state.recordingDate));
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
  if (cameraID !== state.recordingCameraID) return [];
  return state.recordings;
}

async function loadRecordingPage(page = 1, { silent = false } = {}) {
  if (!state.recordingCameraID) return;
  if (!state.recordingDate) {
    state.recordingDate = localDateKey(new Date());
    $('#recordingDate').value = state.recordingDate;
  }
  const range = recordingDateRange();
  if (!range) return;

  const requestCameraID = state.recordingCameraID;
  const requestDate = state.recordingDate;
  const params = new URLSearchParams({
    camera_id: requestCameraID,
    page: String(Math.max(1, Number(page) || 1)),
    page_size: String(Math.max(1, Number(state.recordingPage.page_size) || 24)),
    from: range.from,
    to: range.to,
  });

  try {
    const payload = await api('/api/recordings?' + params.toString());
    if (state.recordingCameraID !== requestCameraID || state.recordingDate !== requestDate) return;

    state.recordings = payload?.timeline || [];
    state.recordingPage = {
      items: payload?.items || [],
      page: Math.max(1, Number(payload?.page) || 1),
      page_size: Math.max(1, Number(payload?.page_size) || 24),
      total: Math.max(0, Number(payload?.total) || 0),
      total_pages: Math.max(0, Number(payload?.total_pages) || 0),
    };

    renderRecordingStats();
    renderRecordingTimeline(state.recordings);
    renderObjectTimeline(state.recordings);
    renderRecordingCards(state.recordingPage.items);
    renderRecordingPagination();
    loadRecordingAnalytics({ silent: true });
    loadAnalyticsSummary({ silent: true, force: true });
  } catch (error) {
    if (!(error instanceof SessionRequiredError) && !silent) setStatus(error.message, false);
  }
}

function objectSegmentsFingerprint(segments) {
  return JSON.stringify((segments || []).map(segment => [
    segment.id, segment.recording_name, segment.class_id, segment.class_name,
    segment.start_seconds, segment.end_seconds, segment.peak_seconds,
    segment.max_confidence, segment.best_score,
  ]));
}

async function loadRecordingAnalytics({ silent = false } = {}) {
  if (state.analyticsLoading || !state.recordingCameraID) return;
  state.analyticsLoading = true;
  try {
    const cameraID = encodeURIComponent(state.recordingCameraID);
    const [status, segments, classes] = await Promise.all([
      api('/api/analytics/status'),
      api(`/api/analytics/objects?camera_id=${cameraID}`),
      state.analyticsClasses.length ? Promise.resolve(state.analyticsClasses) : api('/api/analytics/classes'),
    ]);
    state.analyticsClasses = classes || [];
    const nextSegments = segments || [];
    const changed = objectSegmentsFingerprint(nextSegments) !== objectSegmentsFingerprint(state.objectSegments);
    state.analyticsStatus = status;
    state.objectSegments = nextSegments;
    state.personSegments = nextSegments.filter(segment => segment.class_name === 'person');
    renderAnalyticsStatus();
    if (changed) {
      const recordings = recordingsForCamera(state.recordingCameraID);
      renderObjectTimeline(recordings);
      renderRecordingCards(state.recordingPage.items || []);
    }
    await loadAnalyticsSummary({ silent: true });
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
    const count = state.objectSegments.length;
    const classes = new Set(state.objectSegments.map(segment => segment.class_name)).size;
    element.textContent = `NanoDet · 1 FPS · ${count} tramos · ${classes} clases`;
  }
  element.title = 'Detección local multiclase con NanoDet-Plus-m 320.';
}

async function loadAnalyticsSummary({ silent = false, force = false } = {}) {
  const status = $('#analyticsSummaryStatus');
  if (!state.recordingCameraID || !state.recordingDate || !status) return;
  const cacheKey = `${state.recordingCameraID}|${state.recordingDate}`;
  if (!force && state.analyticsSummaryKey === cacheKey && Date.now() - state.analyticsSummaryFetchedAt < 15000) return;
  status.textContent = 'Cargando…';
  status.classList.remove('unavailable');
  try {
    const query = new URLSearchParams({
      camera_id: state.recordingCameraID,
      date: state.recordingDate,
      offset_minutes: String(new Date().getTimezoneOffset()),
    });
    state.analyticsSummary = await api(`/api/analytics/summary?${query}`);
    state.analyticsSummaryKey = cacheKey;
    state.analyticsSummaryFetchedAt = Date.now();
    renderAnalyticsSummary();
  } catch (error) {
    state.analyticsSummary = null;
    status.textContent = 'No disponible';
    status.classList.add('unavailable');
    const grid = $('#analyticsSummaryGrid');
    const classes = $('#analyticsSummaryClasses');
    if (grid) grid.replaceChildren();
    if (classes) classes.replaceChildren();
    if (!(error instanceof SessionRequiredError) && !silent) setStatus(error.message, false);
  }
}

function renderAnalyticsSummary() {
  const summary = state.analyticsSummary;
  const status = $('#analyticsSummaryStatus');
  const grid = $('#analyticsSummaryGrid');
  const classes = $('#analyticsSummaryClasses');
  if (!summary || !status || !grid || !classes) return;
  status.classList.remove('unavailable');
  status.textContent = summary.total_objects > 0 ? `${summary.total_objects} objetos` : 'Sin actividad';
  grid.replaceChildren();
  classes.replaceChildren();

  const busiest = Number(summary.busiest_count) > 0
    ? `${String(Number(summary.busiest_hour) || 0).padStart(2, '0')}:00`
    : '—';
  const stats = [
    ['Objetos', Number(summary.total_objects) || 0],
    ['Hora pico', busiest],
    ['Entradas', Number(summary.zone_entries) || 0],
    ['Salidas', Number(summary.zone_exits) || 0],
    ['Cruces', Number(summary.line_crossings) || 0],
    ['Reglas', Number(summary.rule_matches) || 0],
  ];
  for (const [label, value] of stats) {
    const item = document.createElement('div');
    const strong = document.createElement('strong');
    const span = document.createElement('span');
    strong.textContent = String(value);
    span.textContent = label;
    item.append(strong, span);
    grid.append(item);
  }

  for (const item of (summary.classes || []).slice(0, 10)) {
    const chip = document.createElement('span');
    chip.className = 'analytics-summary-chip';
    const label = item.label || analyticsClassLabel(item.class_name);
    chip.textContent = `${label} ${Number(item.count) || 0}`;
    classes.append(chip);
  }
  if (!(summary.classes || []).length) {
    const empty = document.createElement('span');
    empty.className = 'analytics-summary-empty';
    empty.textContent = 'Aún no hay clases detectadas en esta fecha.';
    classes.append(empty);
  }
}

async function reindexAnalytics() {
  const button = $('#analyticsReindexBtn');
  button.disabled = true;
  try {
    await api('/api/analytics/reindex', { method: 'POST', body: '{}' });
    setStatus('Reanálisis multiclase agregado a la cola', true);
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
    const summary = recordingSummary(camera.id);
    const countValue = Math.max(0, Number(summary?.count) || 0);
    const sizeValue = Math.max(0, Number(summary?.size_bytes) || 0);
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'recording-camera-item';
    button.classList.toggle('active', camera.id === state.recordingCameraID);
    const details = document.createElement('div');
    const name = document.createElement('strong');
    const meta = document.createElement('span');
    const count = document.createElement('span');
    name.textContent = camera.name;
    meta.textContent = countValue ? formatBytes(sizeValue) + ' · ' + countValue + ' clips' : 'Sin grabaciones';
    count.className = 'recording-camera-count';
    count.textContent = String(countValue);
    details.append(name, meta);
    button.append(details, count);
    button.addEventListener('click', () => selectRecordingCamera(camera.id, false, true));
    list.append(button);
  }
}

async function selectRecordingCamera(cameraID, preserveDate = false, updateRoute = false) {
  if (!cameraID) return;
  const cameraChanged = state.recordingCameraID !== cameraID;
  state.recordingCameraID = cameraID;
  state.recordingName = '';
  if (cameraChanged) {
    state.personSegments = [];
    state.objectSegments = [];
    state.analyticsSummary = null;
    state.analyticsSummaryKey = '';
    state.analyticsSummaryFetchedAt = 0;
    state.recordingPage = { items: [], page: 1, page_size: 24, total: 0, total_pages: 0 };
  }
  if (updateRoute) setBrowserRoute('/grabaciones/' + encodeURIComponent(cameraID));
  const camera = state.cameras.find(item => item.id === cameraID);
  const summary = recordingSummary(cameraID);
  if (!preserveDate || !state.recordingDate) {
    state.recordingDate = summary?.latest_started_at
      ? localDateKey(summary.latest_started_at)
      : localDateKey(new Date());
  }
  $('#recordingDate').value = state.recordingDate;
  $('#recordingCameraTitle').textContent = camera?.name || 'Grabaciones';
  renderRecordingCameraList();
  renderRecordingStats();
  await loadRecordingPage(1);
}

function renderRecordingStats() {
  const summary = recordingSummary(state.recordingCameraID);
  const count = Math.max(0, Number(summary?.count) || 0);
  const size = Math.max(0, Number(summary?.size_bytes) || 0);
  const duration = Math.max(0, Number(summary?.duration_seconds) || 0);
  const totalBytes = Math.max(0, Number(state.recordingStorage?.total_bytes) || 0);
  const totalFiles = Math.max(0, Number(state.recordingStorage?.file_count) || 0);
  $('#recordingCount').textContent = String(count);
  $('#recordingSize').textContent = formatBytes(size);
  $('#recordingStorageTotal').textContent = formatBytes(totalBytes);
  $('#recordingStorageMeta').textContent = 'total guardado · ' + totalFiles + ' ' + (totalFiles === 1 ? 'clip' : 'clips');
  $('#recordingDuration').textContent = formatDuration(duration);
  updateRecordingDeleteAction();
}

function localDateFromKey(key) {
  const parts = String(key || '').split('-').map(Number);
  if (parts.length !== 3 || parts.some(value => !Number.isFinite(value))) return null;
  return new Date(parts[0], parts[1] - 1, parts[2], 0, 0, 0, 0);
}

function addLocalDays(date, days) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days, 0, 0, 0, 0);
}

function recordingDeleteFilter() {
  const scope = $('#recordingDeleteScope').value;
  const selected = localDateFromKey(state.recordingDate) || new Date();
  if (scope === 'all') return { all: true, label: 'todas las grabaciones' };
  if (scope === 'day') {
    return { all: false, from: selected, to: addLocalDays(selected, 1), label: `el día ${selected.toLocaleDateString()}` };
  }
  if (scope === 'week') {
    const mondayOffset = (selected.getDay() + 6) % 7;
    const from = addLocalDays(selected, -mondayOffset);
    return { all: false, from, to: addLocalDays(from, 7), label: `la semana del ${from.toLocaleDateString()}` };
  }
  if (scope === 'month') {
    const from = new Date(selected.getFullYear(), selected.getMonth(), 1);
    const to = new Date(selected.getFullYear(), selected.getMonth() + 1, 1);
    return { all: false, from, to, label: from.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }) };
  }
  if (scope === 'previous-month') {
    const now = new Date();
    const from = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const to = new Date(now.getFullYear(), now.getMonth(), 1);
    return { all: false, from, to, label: `el mes anterior (${from.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })})` };
  }
  if (scope === 'custom') {
    const from = localDateFromKey($('#recordingDeleteFrom').value);
    const endDay = localDateFromKey($('#recordingDeleteTo').value);
    if (!from || !endDay || endDay < from) return null;
    return { all: false, from, to: addLocalDays(endDay, 1), label: `del ${from.toLocaleDateString()} al ${endDay.toLocaleDateString()}` };
  }
  return null;
}

function syncRecordingDeleteScope() {
  const custom = $('#recordingDeleteScope').value === 'custom';
  $('#recordingDeleteCustomRange').hidden = !custom;
  if (custom) {
    const fallback = state.recordingDate || localDateKey(new Date());
    if (!$('#recordingDeleteFrom').value) $('#recordingDeleteFrom').value = fallback;
    if (!$('#recordingDeleteTo').value) $('#recordingDeleteTo').value = fallback;
  }
  updateRecordingDeleteAction();
}

function updateRecordingDeleteAction() {
  const button = $('#recordingsDeleteFilteredBtn');
  const allButton = $('#recordingsDeleteAllBtn');
  if (!button || !allButton) return;
  const filter = recordingDeleteFilter();
  const total = Math.max(0, Number(recordingSummary(state.recordingCameraID)?.count) || 0);
  button.disabled = !state.recordingCameraID || !filter || total === 0;
  button.textContent = 'Eliminar filtrados';
  allButton.disabled = !state.recordingCameraID || total === 0;
  allButton.textContent = total > 0 ? 'Eliminar todos (' + total + ')' : 'Eliminar todos';
}

async function deleteCurrentRecording() {
  const recording = state.playback.recording;
  if (!recording) return;
  if (recording.active) {
    setStatus('No se puede eliminar una grabación que todavía está activa.', false);
    return;
  }
  const accepted = await confirmAction({
    title: 'Eliminar grabación',
    message: `Se eliminará permanentemente el video del ${new Date(recording.started_at).toLocaleString()} (${formatDuration(recording.duration_seconds)}). Esta acción no se puede deshacer.`,
    confirmLabel: 'Eliminar video',
  });
  if (!accepted) return;
  const button = $('#recordingDeleteBtn');
  button.disabled = true;
  try {
    await api(recordingBaseURL(recording), { method: 'DELETE' });
    const cameraID = recording.camera_id;
    stopRecordingPlayback();
    state.recordingName = '';
    navigate('recordings', { path: '/grabaciones/' + encodeURIComponent(cameraID), replace: true });
    setStatus('Grabación eliminada', true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    button.disabled = false;
  }
}

async function deleteRecordingsByFilter(filter, button) {
  const total = Math.max(0, Number(recordingSummary(state.recordingCameraID)?.count) || 0);
  if (!filter || !total) {
    setStatus('No hay grabaciones que coincidan con ese filtro.', false);
    return;
  }
  const camera = state.cameras.find(item => item.id === state.recordingCameraID);
  const accepted = await confirmAction({
    title: filter.all ? 'Eliminar todas las grabaciones' : 'Eliminar grabaciones filtradas',
    message: 'Se eliminarán permanentemente los clips cerrados de ' + (camera?.name || 'esta cámara') + ' correspondientes a ' + filter.label + '. Esta acción no se puede deshacer.',
    confirmLabel: 'Eliminar videos',
  });
  if (!accepted) return;

  button.disabled = true;
  try {
    const payload = filter.all
      ? { camera_id: state.recordingCameraID, all: true }
      : { camera_id: state.recordingCameraID, all: false, from: filter.from.toISOString(), to: filter.to.toISOString() };
    const result = await api('/api/recordings/delete-batch', { method: 'POST', body: JSON.stringify(payload) });
    const skipped = Number(result.skipped_active) || 0;
    const suffix = skipped ? ' · ' + skipped + ' activa' + (skipped === 1 ? '' : 's') + ' conservada' + (skipped === 1 ? '' : 's') : '';
    await loadRecordings();
    refreshEvents(false);
    setStatus((result.deleted_count || 0) + ' grabaciones eliminadas' + suffix, true);
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) setStatus(error.message, false);
  } finally {
    updateRecordingDeleteAction();
  }
}

async function deleteFilteredRecordings() {
  await deleteRecordingsByFilter(recordingDeleteFilter(), $('#recordingsDeleteFilteredBtn'));
}

async function deleteAllRecordings() {
  await deleteRecordingsByFilter({ all: true, label: 'todas las grabaciones' }, $('#recordingsDeleteAllBtn'));
}

const TIMELINE_ZOOM_MIN = 1;
const TIMELINE_ZOOM_MAX = 24;
const timelinePointers = new Map();
let timelinePinchStart = null;
let timelineSuppressClickUntil = 0;

function clampTimelineZoom(value) {
  return Math.max(TIMELINE_ZOOM_MIN, Math.min(TIMELINE_ZOOM_MAX, Number(value) || TIMELINE_ZOOM_MIN));
}

function timelineTickMinutes() {
  const zoom = state.timelineZoom;
  if (zoom >= 16) return 15;
  if (zoom >= 8) return 30;
  if (zoom >= 4) return 60;
  if (zoom >= 2) return 120;
  return 240;
}

function timelineVisibleHoursLabel() {
  const hours = 24 / state.timelineZoom;
  const rounded = hours >= 10 ? Math.round(hours) : Math.round(hours * 10) / 10;
  return `${rounded} h`;
}

function renderTimelineRuler() {
  const ruler = $('#timelineRuler');
  const canvas = $('#timelineCanvas');
  if (!ruler || !canvas) return;
  const stepMinutes = timelineTickMinutes();
  canvas.style.setProperty('--timeline-grid-step', `${stepMinutes / (24 * 60) * 100}%`);
  ruler.replaceChildren();
  for (let minute = 0; minute <= 24 * 60; minute += stepMinutes) {
    const tick = document.createElement('span');
    const hour = Math.floor(minute / 60);
    const minutePart = minute % 60;
    tick.textContent = minute === 24 * 60
      ? '24:00'
      : `${String(hour).padStart(2, '0')}:${String(minutePart).padStart(2, '0')}`;
    tick.style.left = `${minute / (24 * 60) * 100}%`;
    if (minute === 0) tick.classList.add('first');
    if (minute === 24 * 60) tick.classList.add('last');
    ruler.append(tick);
  }
}

function syncTimelineZoomUI() {
  const canvas = $('#timelineCanvas');
  if (!canvas) return;
  state.timelineZoom = clampTimelineZoom(state.timelineZoom);
  canvas.style.width = `${state.timelineZoom * 100}%`;
  renderTimelineRuler();
  const label = $('#timelineZoomLabel');
  if (label) label.textContent = timelineVisibleHoursLabel();
  const out = $('#timelineZoomOutBtn');
  const inside = $('#timelineZoomInBtn');
  if (out) out.disabled = state.timelineZoom <= TIMELINE_ZOOM_MIN + .001;
  if (inside) inside.disabled = state.timelineZoom >= TIMELINE_ZOOM_MAX - .001;
}

function setTimelineZoom(value, anchorClientX = null) {
  const viewport = $('#timelineViewport');
  const canvas = $('#timelineCanvas');
  if (!viewport || !canvas) return;
  const nextZoom = clampTimelineZoom(value);
  if (Math.abs(nextZoom - state.timelineZoom) < .001) return;

  const rect = viewport.getBoundingClientRect();
  const anchor = anchorClientX === null
    ? viewport.clientWidth / 2
    : Math.max(0, Math.min(viewport.clientWidth, anchorClientX - rect.left));
  const oldWidth = Math.max(1, viewport.scrollWidth);
  const relativePoint = (viewport.scrollLeft + anchor) / oldWidth;

  state.timelineZoom = nextZoom;
  syncTimelineZoomUI();
  const newWidth = Math.max(1, viewport.scrollWidth);
  viewport.scrollLeft = Math.max(0, relativePoint * newWidth - anchor);
}

function timelinePointerDistance(points) {
  if (points.length < 2) return 0;
  return Math.hypot(points[0].x - points[1].x, points[0].y - points[1].y);
}

function timelinePointerMidpoint(points) {
  if (points.length < 2) return 0;
  return (points[0].x + points[1].x) / 2;
}

function handleTimelinePointerDown(event) {
  if (event.pointerType !== 'touch') return;
  const viewport = $('#timelineViewport');
  if (!viewport) return;
  timelinePointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
  try { viewport.setPointerCapture(event.pointerId); } catch (_) {}
  if (timelinePointers.size === 2) {
    const points = [...timelinePointers.values()];
    timelinePinchStart = {
      distance: Math.max(1, timelinePointerDistance(points)),
      zoom: state.timelineZoom,
    };
  }
}

function handleTimelinePointerMove(event) {
  if (event.pointerType !== 'touch' || !timelinePointers.has(event.pointerId)) return;
  const viewport = $('#timelineViewport');
  if (!viewport) return;
  const previous = timelinePointers.get(event.pointerId);
  timelinePointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
  const points = [...timelinePointers.values()];

  if (points.length >= 2) {
    if (!timelinePinchStart) {
      timelinePinchStart = { distance: Math.max(1, timelinePointerDistance(points)), zoom: state.timelineZoom };
    }
    const distance = timelinePointerDistance(points);
    if (distance > 0) {
      event.preventDefault();
      timelineSuppressClickUntil = Date.now() + 300;
      setTimelineZoom(timelinePinchStart.zoom * distance / timelinePinchStart.distance, timelinePointerMidpoint(points));
    }
    return;
  }

  const dx = event.clientX - previous.x;
  const dy = event.clientY - previous.y;
  if (state.timelineZoom > TIMELINE_ZOOM_MIN && Math.abs(dx) > 1 && Math.abs(dx) > Math.abs(dy)) {
    event.preventDefault();
    timelineSuppressClickUntil = Date.now() + 250;
    viewport.scrollLeft -= dx;
  }
}

function handleTimelinePointerEnd(event) {
  if (event.pointerType !== 'touch') return;
  timelinePointers.delete(event.pointerId);
  if (timelinePointers.size < 2) timelinePinchStart = null;
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
  syncTimelineZoomUI();
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

function analyticsClassLabel(className) {
  const found = state.analyticsClasses.find(item => item.name === className);
  if (found?.label) return found.label;
  return String(className || 'Objeto').replace(/\b\w/g, char => char.toUpperCase());
}

function renderObjectTimeline(recordings) {
  const tracks = $('#analyticsTracks');
  const labels = $('#analyticsTrackLabels');
  if (!tracks || !labels) return;
  tracks.replaceChildren();
  labels.replaceChildren();
  const bounds = selectedDateBounds();
  if (!bounds) return;
  const byName = new Map(recordings.map(recording => [recording.name, recording]));
  const visibleSegments = state.objectSegments.filter(segment => {
    const recording = byName.get(segment.recording_name);
    if (!recording) return false;
    const recordingStart = new Date(recording.started_at).getTime();
    if (!Number.isFinite(recordingStart)) return false;
    const start = recordingStart + Math.max(0, Number(segment.start_seconds) || 0) * 1000;
    const end = recordingStart + Math.max(Number(segment.end_seconds) || 0, Number(segment.start_seconds) || 0) * 1000;
    return end > bounds.start && start < bounds.end;
  });
  const classes = [...new Map(visibleSegments.map(segment => [segment.class_name, segment])).values()]
    .sort((a, b) => (Number(a.class_id) || 0) - (Number(b.class_id) || 0));

  if (!classes.length) {
    const label = document.createElement('span');
    label.textContent = 'Objetos';
    labels.append(label);
    const empty = document.createElement('div');
    empty.className = 'analytics-object-track empty';
    empty.textContent = 'Sin objetos detectados';
    tracks.append(empty);
    return;
  }

  for (const classSegment of classes) {
    const className = classSegment.class_name;
    const label = document.createElement('span');
    label.textContent = analyticsClassLabel(className);
    label.title = analyticsClassLabel(className);
    labels.append(label);

    const timeline = document.createElement('div');
    timeline.className = 'analytics-object-track';
    timeline.dataset.className = className;
    for (const segment of visibleSegments.filter(item => item.class_name === className)) {
      const recording = byName.get(segment.recording_name);
      const recordingStart = new Date(recording.started_at).getTime();
      const start = recordingStart + Math.max(0, Number(segment.start_seconds) || 0) * 1000;
      const end = recordingStart + Math.max(Number(segment.end_seconds) || 0, Number(segment.start_seconds) || 0) * 1000;
      const geometry = timelineGeometry(start, Math.max(start + 1000, end), bounds);
      if (!geometry) continue;
      const marker = document.createElement('button');
      marker.type = 'button';
      marker.className = 'analytics-object-segment';
      marker.style.left = `${geometry.left}%`;
      marker.style.width = `${geometry.width}%`;
      const confidence = Math.round((Number(segment.max_confidence) || 0) * 100);
      const moment = new Date(recordingStart + (Number(segment.peak_seconds) || 0) * 1000);
      const title = `${analyticsClassLabel(className)} · ${moment.toLocaleTimeString()} · ${confidence}%`;
      marker.title = title;
      marker.setAttribute('aria-label', `Abrir ${title}`);
      marker.addEventListener('click', () => selectRecording(recording, Number(segment.peak_seconds) || 0));
      timeline.append(marker);
    }
    tracks.append(timeline);
  }
}

function bestObjectSegmentForRecording(recording) {
  let best = null;
  for (const segment of state.objectSegments) {
    if (segment.camera_id !== recording.camera_id || segment.recording_name !== recording.name) continue;
    const score = Number(segment.best_score) || Number(segment.max_confidence) || 0;
    const bestValue = best ? (Number(best.best_score) || Number(best.max_confidence) || 0) : -1;
    if (!best || score > bestValue) best = segment;
  }
  return best;
}

function renderRecordingCards(recordings) {
  const list = $('#recordingList');
  list.replaceChildren();
  const page = state.recordingPage;
  const first = page.total ? (page.page - 1) * page.page_size + 1 : 0;
  const last = page.total ? Math.min(page.total, first + recordings.length - 1) : 0;
  $('#recordingListMeta').textContent = page.total
    ? first + '–' + last + ' de ' + page.total + ' en ' + state.recordingDate
    : '0 grabaciones en ' + state.recordingDate;

  if (!recordings.length) {
    const empty = document.createElement('div');
    empty.className = 'placeholder-panel';
    empty.innerHTML = '<div class="empty-icon" aria-hidden="true">◫</div><h2>Sin clips en esta fecha</h2><p>Elige otra fecha o espera a que la cámara genere nuevas grabaciones.</p>';
    list.append(empty);
    return;
  }

  for (const recording of recordings) {
    const card = document.createElement('article');
    card.className = 'recording-card';
    card.classList.toggle('active', state.playback.recording?.name === recording.name && state.playback.recording?.camera_id === recording.camera_id);
    const objectSegment = bestObjectSegmentForRecording(recording);
    const preview = document.createElement('button');
    preview.type = 'button';
    preview.className = 'recording-preview loading';
    const image = document.createElement('img');
    const fallback = document.createElement('div');
    const play = document.createElement('span');
    image.alt = objectSegment
      ? `${analyticsClassLabel(objectSegment.class_name)} detectado en la grabación ${new Date(recording.started_at).toLocaleString()}`
      : `Miniatura de grabación ${new Date(recording.started_at).toLocaleString()}`;
    image.loading = 'lazy';
    image.src = recordingSnapshotURL(recording, objectSegment?.peak_seconds);
    fallback.className = 'recording-preview-fallback';
    fallback.hidden = true;
    fallback.innerHTML = '<span aria-hidden="true">◫</span><strong>Sin miniatura</strong>';
    play.className = 'recording-card-play';
    play.innerHTML = '<i aria-hidden="true">▶</i>';
    let thumbnailFallbackTried = false;
    image.addEventListener('load', () => preview.classList.remove('loading'));
    image.addEventListener('error', () => {
      if (objectSegment && !thumbnailFallbackTried) {
        thumbnailFallbackTried = true;
        image.src = recordingSnapshotURL(recording);
        return;
      }
      preview.classList.remove('loading');
      image.hidden = true;
      fallback.hidden = false;
    });
    preview.append(image, fallback, play);
    if (objectSegment) {
      const badge = document.createElement('span');
      badge.className = 'recording-object-badge';
      badge.textContent = `${analyticsClassLabel(objectSegment.class_name).toUpperCase()} ${Math.round((Number(objectSegment.max_confidence) || 0) * 100)}%`;
      preview.append(badge);
    }
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

function recordingPaginationPages(page, totalPages) {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1);
  const wanted = new Set([1, totalPages, page - 1, page, page + 1]);
  const pages = [...wanted].filter(value => value >= 1 && value <= totalPages).sort((a, b) => a - b);
  const result = [];
  for (const value of pages) {
    const previous = result[result.length - 1];
    if (typeof previous === 'number' && value - previous > 1) result.push('…');
    result.push(value);
  }
  return result;
}

function renderRecordingPagination() {
  const nav = $('#recordingPagination');
  const numbers = $('#recordingPageNumbers');
  const status = $('#recordingPageStatus');
  const previous = $('#recordingPagePrev');
  const next = $('#recordingPageNext');
  const page = state.recordingPage;
  if (!nav || !numbers || !status || !previous || !next) return;

  numbers.replaceChildren();
  const totalPages = Math.max(0, Number(page.total_pages) || 0);
  nav.hidden = page.total === 0;
  previous.disabled = totalPages <= 1 || page.page <= 1;
  next.disabled = totalPages <= 1 || page.page >= totalPages;
  status.textContent = totalPages ? 'Página ' + page.page + ' de ' + totalPages : 'Sin resultados';

  for (const value of recordingPaginationPages(page.page, totalPages)) {
    if (value === '…') {
      const ellipsis = document.createElement('span');
      ellipsis.className = 'recording-page-ellipsis';
      ellipsis.textContent = '…';
      numbers.append(ellipsis);
      continue;
    }
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'recording-page-button';
    button.classList.toggle('active', value === page.page);
    button.setAttribute('aria-label', 'Ir a página ' + value);
    if (value === page.page) button.setAttribute('aria-current', 'page');
    button.textContent = String(value);
    button.addEventListener('click', () => loadRecordingPage(value));
    numbers.append(button);
  }
}

function recordingBaseURL(recording) {
  return `/api/recordings/${encodeURIComponent(recording.camera_id)}/${encodeURIComponent(recording.name)}`;
}

function recordingSnapshotURL(recording, offsetSeconds = null) {
  if (offsetSeconds === null || offsetSeconds === undefined || !Number.isFinite(Number(offsetSeconds))) {
    return `${recordingBaseURL(recording)}/snapshot`;
  }
  const offset = Math.max(0, Number(offsetSeconds) || 0);
  return `${recordingBaseURL(recording)}/snapshot?at=${offset.toFixed(3)}`;
}

function recordingPlaybackPath(cameraID, name, offset = 0) {
  const base = '/grabaciones/' + encodeURIComponent(cameraID) + '/' + encodeURIComponent(name);
  const safeOffset = Math.max(0, Number(offset) || 0);
  return safeOffset > 0 ? base + '?at=' + safeOffset.toFixed(3) : base;
}

function selectRecording(recording, offset = 0) {
  if (!recording?.camera_id || !recording?.name) return;
  const recordingStart = new Date(recording.started_at).getTime();
  if (Number.isFinite(recordingStart)) {
    state.recordingDate = localDateKey(new Date(recordingStart + Math.max(0, Number(offset) || 0) * 1000));
  }
  state.recordingCameraID = recording.camera_id;
  state.recordingName = recording.name;
  state.playbackRouteOffset = Math.max(0, Number(offset) || 0);
  navigate('playback', {
    path: recordingPlaybackPath(recording.camera_id, recording.name, state.playbackRouteOffset),
  });
}

async function loadDedicatedPlayback() {
  stopRecordingPlayback();
  const loading = $('#playbackLoading');
  const error = $('#playbackPageError');
  const player = $('#recordingPlayer');
  loading.hidden = false;
  error.hidden = true;
  player.hidden = true;

  const cameraID = state.recordingCameraID;
  const name = state.recordingName;
  if (!cameraID || !name) {
    loading.hidden = true;
    error.hidden = false;
    $('#playbackPageErrorMessage').textContent = 'No se indicó una grabación válida.';
    return;
  }

  const recording = { camera_id: cameraID, name };
  try {
    const metadata = await api(recordingBaseURL(recording) + '/metadata');
    if (state.currentView !== 'playback' || state.recordingCameraID !== cameraID || state.recordingName !== name) return;

    Object.assign(recording, metadata || {});
    const camera = state.cameras.find(item => item.id === cameraID);
    state.recordingDate = localDateKey(recording.started_at);
    $('#recordingPlaybackCamera').textContent = camera?.name || 'Grabación';
    $('#recordingPlayerTitle').textContent = new Date(recording.started_at).toLocaleTimeString();
    $('#recordingPlayerDate').textContent = new Date(recording.started_at).toLocaleString();
    $('#recordingPlayerMeta').textContent = formatDuration(recording.duration_seconds) + ' · ' + formatBytes(recording.size_bytes);
    $('#recordingDownload').href = recordingBaseURL(recording) + '/download';
    $('#recordingDeleteBtn').disabled = Boolean(recording.active);
    $('#recordingDeleteBtn').title = recording.active ? 'La grabación activa no se puede eliminar' : '';
    $('#recordingScrubber').max = String(Math.max(.1, Number(recording.duration_seconds) || .1));
    $('#recordingTotalTime').textContent = formatClock(recording.duration_seconds);
    $('#recordingPlaybackLoading').hidden = false;

    loading.hidden = true;
    player.hidden = false;
    startRecordingPlayback(recording, state.playbackRouteOffset);
  } catch (loadError) {
    loading.hidden = true;
    if (loadError instanceof SessionRequiredError) return;
    error.hidden = false;
    $('#playbackPageErrorMessage').textContent = loadError.message;
    setStatus(loadError.message, false);
  }
}

function backToRecordingLibrary() {
  const cameraID = state.recordingCameraID;
  stopRecordingPlayback();
  state.recordingName = '';
  state.playbackRouteOffset = 0;
  navigate('recordings', {
    path: cameraID ? '/grabaciones/' + encodeURIComponent(cameraID) : '/grabaciones',
  });
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
    if (state.playback.playing) {
      const duration = Math.max(0, Number(state.playback.recording?.duration_seconds) || 0);
      const current = currentPlaybackOffset();
      state.playback.offset = duration > 0 ? Math.min(current, duration) : current;
      state.playback.playing = false;
      if (state.playback.timer !== null) clearInterval(state.playback.timer);
      state.playback.timer = null;
      $('#recordingPlayPause').textContent = 'Reproducir';
      updatePlaybackUI(state.playback.offset);
      // Un cierre al final del MJPEG no es un error visible; una interrupción
      // claramente anterior al final sí se muestra como fallo de reproducción.
      errorBox.hidden = duration > 0 && state.playback.offset >= Math.max(0, duration - .8);
    }
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
  return [
    event.camera_id, event.time, event.topic || '', event.data?.State || '',
    event.object_class || '', event.zone_name || '', event.line_name || '', event.direction || '',
  ].join('|');
}

function eventRecordingSnapshotURL(event, attempt = 0) {
  if (!event.recording_name) return '';
  const hasObject = Number(event.object_confidence) > 0;
  const hasPerson = Number(event.person_confidence) > 0;
  const offsetValue = hasObject ? event.object_offset : (hasPerson ? event.person_offset : event.recording_offset);
  const offset = Math.max(0, Number(offsetValue) || 0);
  return `/api/recordings/${encodeURIComponent(event.camera_id)}/${encodeURIComponent(event.recording_name)}/snapshot?at=${offset.toFixed(3)}&r=${attempt}`;
}

function eventCameraSnapshotURL(event) {
  return `/api/cameras/${encodeURIComponent(event.camera_id)}/snapshot?v=${encodeURIComponent(event.time || Date.now())}`;
}

function eventThumbnailFingerprint(event) {
  return [
    event.recording_name || '',
    Number(event.object_offset) || 0,
    Number(event.object_confidence) || 0,
    Number(event.person_offset) || 0,
    Number(event.person_confidence) || 0,
    event.smart_type || '',
  ].join('|');
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
  const offsetValue = Number(event.object_confidence) > 0
    ? event.object_offset
    : (Number(event.person_confidence) > 0 ? event.person_offset : event.recording_offset);
  const offset = Math.max(0, Number(offsetValue) || 0);
  state.recordingCameraID = event.camera_id;
  state.recordingName = event.recording_name;
  state.playbackRouteOffset = offset;
  navigate('playback', {
    path: recordingPlaybackPath(event.camera_id, event.recording_name, offset),
  });
}

function smartEventLabel(event) {
  if (!event.object_class) return (event.topic || 'Evento').split('/').pop() || 'Evento';
  const object = analyticsClassLabel(event.object_class);
  switch (event.smart_type) {
    case 'zone_enter': return `${object} · entró en ${event.zone_name || 'zona'}`;
    case 'zone_exit': return `${object} · salió de ${event.zone_name || 'zona'}`;
    case 'line_cross': return `${object} · cruzó ${event.line_name || 'línea'}${event.direction ? ` · ${event.direction}` : ''}`;
    default: return `${object} detectado`;
  }
}

function createEventCard(event) {
  const card = document.createElement(event.recording_name ? 'button' : 'article');
  if (card instanceof HTMLButtonElement) card.type = 'button';
  card.className = 'event-card event-enter';
  card.dataset.eventKey = eventKey(event);
  card.dataset.thumbFingerprint = eventThumbnailFingerprint(event);
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
  topic.textContent = smartEventLabel(event);
  const details = document.createElement('span');
  details.className = 'event-detail';
  const confidence = Number(event.object_confidence) || Number(event.person_confidence) || Number(event.data?.Score) || 0;
  const score = confidence > 0 ? ` · ${Math.round(confidence * 100)}%` : '';
  details.textContent = `${new Date(event.time).toLocaleString()}${score}`;
  body.append(camera, topic, details);

  const side = document.createElement('div');
  side.className = 'event-side';
  if (event.recording_name) {
    const action = document.createElement('span');
    action.className = 'event-jump';
    const jumpOffset = Number(event.object_confidence) > 0
      ? event.object_offset
      : (Number(event.person_confidence) > 0 ? event.person_offset : event.recording_offset);
    action.textContent = `▶ ${formatClock(jumpOffset || 0)}`;
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
  const ordered = visible.map(event => {
    const card = existing.get(eventKey(event));
    if (!card) return createEventCard(event);
    const fingerprint = eventThumbnailFingerprint(event);
    if (card.dataset.thumbFingerprint !== fingerprint) {
      card.dataset.thumbFingerprint = fingerprint;
      const image = $('img', card);
      const thumb = $('.event-thumb', card);
      if (image && thumb) {
        thumb.classList.add('loading');
        loadEventThumbnail(image, event);
      }
    }
    return card;
  });
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
  // El medio sólo afecta a las alertas automáticas. La duración también se usa
  // como valor por defecto de /video, por lo que debe seguir editable en foto.
  $('#telegramVideoSeconds').disabled = false;
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

function cloneAnalyticsConfig(config) {
  return JSON.parse(JSON.stringify(config || { enabled_classes: [], zones: [], lines: [], rules: [] }));
}

function analyticsClientID(prefix) {
  if (window.crypto?.randomUUID) return `${prefix}${window.crypto.randomUUID().replaceAll('-', '').slice(0, 12)}`;
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
}

async function openAnalyticsDialog(camera) {
  const dialog = $('#analyticsDialog');
  state.analyticsCameraID = camera.id;
  state.analyticsDrawing = null;
  state.analyticsRuleEditIndex = -1;
  $('#analyticsDialogTitle').textContent = `Analítica · ${camera.name}`;
  $('#analyticsDialogSubtitle').textContent = 'Elige clases, dibuja zonas/líneas y combina reglas sin añadir otra inferencia.';
  $('#analyticsConfigError').textContent = '';
  $('#analyticsConfigMessage').textContent = '';
  $('#analyticsClassSearch').value = '';
  $('#analyticsConfigLoading').hidden = false;
  $('#analyticsConfigContent').hidden = true;
  $('#analyticsSaveBtn').disabled = true;
  dialog.showModal();
  try {
    const cameraID = encodeURIComponent(camera.id);
    const [classes, config] = await Promise.all([
      state.analyticsClasses.length ? Promise.resolve(state.analyticsClasses) : api('/api/analytics/classes'),
      api(`/api/analytics/config/${cameraID}`),
    ]);
    state.analyticsClasses = classes || [];
    state.analyticsConfig = cloneAnalyticsConfig(config);
    $('#analyticsPreviewImage').src = `/api/cameras/${cameraID}/snapshot?v=${Date.now()}`;
    $('#analyticsConfigLoading').hidden = true;
    $('#analyticsConfigContent').hidden = false;
    $('#analyticsSaveBtn').disabled = false;
    renderAnalyticsConfig();
    resetAnalyticsRuleEditor();
  } catch (error) {
    $('#analyticsConfigLoading').hidden = true;
    if (!(error instanceof SessionRequiredError)) $('#analyticsConfigError').textContent = error.message;
  }
}

function renderAnalyticsConfig() {
  renderAnalyticsClassList();
  renderAnalyticsGeometryLists();
  renderAnalyticsOverlay();
  renderAnalyticsRuleSelectors();
  renderAnalyticsRules();
}

function analyticsGroupLabel(group) {
  const labels = { personas: 'Personas', 'vehículos': 'Vehículos', animales: 'Animales', objetos: 'Objetos' };
  return labels[group] || group || 'Objetos';
}

function renderAnalyticsClassList() {
  const list = $('#analyticsClassList');
  if (!list || !state.analyticsConfig) return;
  list.replaceChildren();
  const query = $('#analyticsClassSearch').value.trim().toLowerCase();
  const enabled = new Set(state.analyticsConfig.enabled_classes || []);
  const groups = new Map();
  for (const item of state.analyticsClasses) {
    const label = item.label || item.name;
    const haystack = `${label} ${item.name} ${item.group}`.toLowerCase();
    if (query && !haystack.includes(query)) continue;
    const group = analyticsGroupLabel(item.group);
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(item);
  }
  for (const [groupName, items] of groups) {
    const section = document.createElement('section');
    section.className = 'analytics-class-group';
    const title = document.createElement('strong');
    title.textContent = groupName;
    const grid = document.createElement('div');
    grid.className = 'analytics-class-grid';
    for (const item of items) {
      const label = document.createElement('label');
      label.className = 'analytics-class-option';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.checked = enabled.has(item.name);
      input.addEventListener('change', () => {
        const set = new Set(state.analyticsConfig.enabled_classes || []);
        if (input.checked) set.add(item.name); else set.delete(item.name);
        state.analyticsConfig.enabled_classes = [...set];
        $('#analyticsClassCount').textContent = String(set.size);
        renderAnalyticsRuleSelectors();
      });
      const copy = document.createElement('span');
      const strong = document.createElement('strong');
      const small = document.createElement('small');
      strong.textContent = item.label || item.name;
      small.textContent = item.name;
      copy.append(strong, small);
      label.append(input, copy);
      grid.append(label);
    }
    section.append(title, grid);
    list.append(section);
  }
  $('#analyticsClassCount').textContent = String(enabled.size);
  if (!groups.size) {
    const empty = document.createElement('p');
    empty.className = 'hint analytics-inline-empty';
    empty.textContent = 'No hay clases que coincidan con la búsqueda.';
    list.append(empty);
  }
}

function renderAnalyticsGeometryLists() {
  if (!state.analyticsConfig) return;
  const zoneList = $('#analyticsZoneList');
  const lineList = $('#analyticsLineList');
  zoneList.replaceChildren();
  lineList.replaceChildren();
  const renderItem = (item, index, type) => {
    const row = document.createElement('div');
    row.className = 'analytics-geometry-item';
    const input = document.createElement('input');
    input.value = item.name || `${type === 'zone' ? 'Zona' : 'Línea'} ${index + 1}`;
    input.maxLength = 64;
    input.setAttribute('aria-label', `Nombre de ${type === 'zone' ? 'zona' : 'línea'}`);
    input.addEventListener('input', () => { item.name = input.value.slice(0, 64); renderAnalyticsOverlay(); renderAnalyticsRuleSelectors(); });
    const remove = document.createElement('button');
    remove.type = 'button';
    remove.className = 'icon-button';
    remove.textContent = '×';
    remove.title = `Eliminar ${type === 'zone' ? 'zona' : 'línea'}`;
    remove.addEventListener('click', () => {
      const collection = type === 'zone' ? state.analyticsConfig.zones : state.analyticsConfig.lines;
      const removedID = collection[index]?.id;
      collection.splice(index, 1);
      state.analyticsConfig.rules = (state.analyticsConfig.rules || []).filter(rule => type === 'zone' ? rule.zone_id !== removedID : rule.line_id !== removedID);
      renderAnalyticsGeometryLists();
      renderAnalyticsOverlay();
      renderAnalyticsRuleSelectors();
      renderAnalyticsRules();
      resetAnalyticsRuleEditor();
    });
    row.append(input, remove);
    return row;
  };
  (state.analyticsConfig.zones || []).forEach((zone, index) => zoneList.append(renderItem(zone, index, 'zone')));
  (state.analyticsConfig.lines || []).forEach((line, index) => lineList.append(renderItem(line, index, 'line')));
  $('#analyticsZoneCount').textContent = String((state.analyticsConfig.zones || []).length);
  $('#analyticsLineCount').textContent = String((state.analyticsConfig.lines || []).length);
  if (!state.analyticsConfig.zones?.length) {
    const empty = document.createElement('span'); empty.className = 'hint'; empty.textContent = 'Sin zonas'; zoneList.append(empty);
  }
  if (!state.analyticsConfig.lines?.length) {
    const empty = document.createElement('span'); empty.className = 'hint'; empty.textContent = 'Sin líneas'; lineList.append(empty);
  }
}

function svgPoint(point) {
  return `${Math.round(point.x * 1000)},${Math.round(point.y * 1000)}`;
}

function renderAnalyticsOverlay() {
  const svg = $('#analyticsOverlay');
  if (!svg || !state.analyticsConfig) return;
  svg.replaceChildren();
  const ns = 'http://www.w3.org/2000/svg';
  for (const zone of state.analyticsConfig.zones || []) {
    const polygon = document.createElementNS(ns, 'polygon');
    polygon.setAttribute('points', (zone.points || []).map(svgPoint).join(' '));
    polygon.setAttribute('class', 'analytics-zone-shape');
    svg.append(polygon);
    if (zone.points?.length) {
      const label = document.createElementNS(ns, 'text');
      label.setAttribute('x', String(zone.points[0].x * 1000 + 10));
      label.setAttribute('y', String(zone.points[0].y * 1000 + 28));
      label.setAttribute('class', 'analytics-overlay-label');
      label.textContent = zone.name || 'Zona';
      svg.append(label);
    }
  }
  for (const line of state.analyticsConfig.lines || []) {
    const element = document.createElementNS(ns, 'line');
    element.setAttribute('x1', String(line.a.x * 1000));
    element.setAttribute('y1', String(line.a.y * 1000));
    element.setAttribute('x2', String(line.b.x * 1000));
    element.setAttribute('y2', String(line.b.y * 1000));
    element.setAttribute('class', 'analytics-line-shape');
    svg.append(element);
    const label = document.createElementNS(ns, 'text');
    label.setAttribute('x', String((line.a.x + line.b.x) * 500 + 10));
    label.setAttribute('y', String((line.a.y + line.b.y) * 500 - 10));
    label.setAttribute('class', 'analytics-overlay-label');
    label.textContent = line.name || 'Línea';
    svg.append(label);
  }
  const drawing = state.analyticsDrawing;
  if (drawing?.points?.length) {
    const preview = document.createElementNS(ns, 'polyline');
    preview.setAttribute('points', drawing.points.map(svgPoint).join(' '));
    preview.setAttribute('class', 'analytics-drawing-shape');
    svg.append(preview);
    for (const point of drawing.points) {
      const dot = document.createElementNS(ns, 'circle');
      dot.setAttribute('cx', String(point.x * 1000));
      dot.setAttribute('cy', String(point.y * 1000));
      dot.setAttribute('r', '10');
      dot.setAttribute('class', 'analytics-drawing-point');
      svg.append(dot);
    }
  }
}

function startAnalyticsDrawing(type) {
  state.analyticsDrawing = { type, points: [] };
  $('#analyticsDrawZoneBtn').disabled = true;
  $('#analyticsDrawLineBtn').disabled = true;
  $('#analyticsCancelDrawBtn').hidden = false;
  $('#analyticsFinishDrawBtn').hidden = type !== 'zone';
  $('#analyticsFinishDrawBtn').disabled = true;
  $('#analyticsDrawHint').textContent = type === 'zone'
    ? 'Toca al menos 3 puntos para formar el polígono y luego pulsa Terminar.'
    : 'Toca dos puntos para crear la línea de cruce.';
  $('#analyticsPreviewWrap').classList.add('drawing');
  renderAnalyticsOverlay();
}

function cancelAnalyticsDrawing() {
  state.analyticsDrawing = null;
  $('#analyticsDrawZoneBtn').disabled = false;
  $('#analyticsDrawLineBtn').disabled = false;
  $('#analyticsCancelDrawBtn').hidden = true;
  $('#analyticsFinishDrawBtn').hidden = true;
  $('#analyticsFinishDrawBtn').disabled = true;
  $('#analyticsPreviewWrap').classList.remove('drawing');
  $('#analyticsDrawHint').textContent = 'Dibuja zonas o líneas sobre la imagen. Las coordenadas se guardan normalizadas, por lo que siguen funcionando aunque cambie el tamaño de pantalla.';
  renderAnalyticsOverlay();
}

function finishAnalyticsDrawing() {
  const drawing = state.analyticsDrawing;
  if (!drawing || !state.analyticsConfig) return;
  if (drawing.type === 'zone') {
    if (drawing.points.length < 3) return;
    state.analyticsConfig.zones ||= [];
    state.analyticsConfig.zones.push({ id: analyticsClientID('z'), name: `Zona ${state.analyticsConfig.zones.length + 1}`, points: drawing.points });
  } else if (drawing.type === 'line' && drawing.points.length >= 2) {
    state.analyticsConfig.lines ||= [];
    state.analyticsConfig.lines.push({ id: analyticsClientID('l'), name: `Línea ${state.analyticsConfig.lines.length + 1}`, a: drawing.points[0], b: drawing.points[1] });
  }
  cancelAnalyticsDrawing();
  renderAnalyticsGeometryLists();
  renderAnalyticsRuleSelectors();
  renderAnalyticsRules();
}

function handleAnalyticsOverlayPoint(event) {
  const drawing = state.analyticsDrawing;
  if (!drawing) return;
  const rect = $('#analyticsOverlay').getBoundingClientRect();
  if (!rect.width || !rect.height) return;
  const point = {
    x: Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)),
    y: Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height)),
  };
  drawing.points.push(point);
  renderAnalyticsOverlay();
  if (drawing.type === 'zone') {
    $('#analyticsFinishDrawBtn').disabled = drawing.points.length < 3;
  } else if (drawing.points.length >= 2) {
    finishAnalyticsDrawing();
  }
}

function renderAnalyticsRuleSelectors() {
  if (!state.analyticsConfig) return;
  const classSelect = $('#analyticsRuleClass');
  const zoneSelect = $('#analyticsRuleZone');
  const lineSelect = $('#analyticsRuleLine');
  const previousClass = classSelect.value;
  const previousZone = zoneSelect.value;
  const previousLine = lineSelect.value;
  classSelect.replaceChildren();
  const all = document.createElement('option'); all.value = '*'; all.textContent = 'Cualquier objeto'; classSelect.append(all);
  const enabled = new Set(state.analyticsConfig.enabled_classes || []);
  for (const item of state.analyticsClasses.filter(item => enabled.has(item.name))) {
    const option = document.createElement('option'); option.value = item.name; option.textContent = item.label || item.name; classSelect.append(option);
  }
  zoneSelect.replaceChildren();
  for (const zone of state.analyticsConfig.zones || []) {
    const option = document.createElement('option'); option.value = zone.id; option.textContent = zone.name; zoneSelect.append(option);
  }
  lineSelect.replaceChildren();
  for (const line of state.analyticsConfig.lines || []) {
    const option = document.createElement('option'); option.value = line.id; option.textContent = line.name; lineSelect.append(option);
  }
  if ([...classSelect.options].some(option => option.value === previousClass)) classSelect.value = previousClass;
  if ([...zoneSelect.options].some(option => option.value === previousZone)) zoneSelect.value = previousZone;
  if ([...lineSelect.options].some(option => option.value === previousLine)) lineSelect.value = previousLine;
  syncAnalyticsRuleTypeFields();
}

function syncAnalyticsRuleTypeFields() {
  const type = $('#analyticsRuleType').value;
  $('#analyticsRuleZoneField').hidden = type !== 'zone_enter' && type !== 'zone_exit';
  $('#analyticsRuleLineField').hidden = type !== 'line_cross';
}

function resetAnalyticsRuleEditor() {
  state.analyticsRuleEditIndex = -1;
  $('#analyticsRuleName').value = '';
  $('#analyticsRuleClass').value = '*';
  $('#analyticsRuleType').value = 'object';
  $('#analyticsRuleStart').value = '';
  $('#analyticsRuleEnd').value = '';
  $('#analyticsRuleCooldown').value = '30';
  $('#analyticsRuleTelegram').checked = false;
  for (const input of $$('#analyticsRuleDays input[type="checkbox"]')) input.checked = false;
  $('#analyticsRuleAddBtn').textContent = 'Agregar regla';
  syncAnalyticsRuleTypeFields();
}

function editAnalyticsRule(index) {
  const rule = state.analyticsConfig?.rules?.[index];
  if (!rule) return;
  state.analyticsRuleEditIndex = index;
  renderAnalyticsRuleSelectors();
  $('#analyticsRuleName').value = rule.name || '';
  $('#analyticsRuleClass').value = rule.class_name || '*';
  $('#analyticsRuleType').value = rule.event_type || 'object';
  $('#analyticsRuleZone').value = rule.zone_id || '';
  $('#analyticsRuleLine').value = rule.line_id || '';
  $('#analyticsRuleStart').value = rule.start || '';
  $('#analyticsRuleEnd').value = rule.end || '';
  $('#analyticsRuleCooldown').value = String(rule.cooldown_seconds || 30);
  $('#analyticsRuleTelegram').checked = Boolean(rule.telegram);
  const days = new Set((rule.days || []).map(Number));
  for (const input of $$('#analyticsRuleDays input[type="checkbox"]')) input.checked = days.has(Number(input.value));
  $('#analyticsRuleAddBtn').textContent = 'Guardar regla';
  syncAnalyticsRuleTypeFields();
}

function renderAnalyticsRules() {
  const list = $('#analyticsRuleList');
  if (!list || !state.analyticsConfig) return;
  list.replaceChildren();
  const rules = state.analyticsConfig.rules || [];
  $('#analyticsRuleCount').textContent = String(rules.length);
  rules.forEach((rule, index) => {
    const row = document.createElement('div');
    row.className = 'analytics-rule-item';
    const enabled = document.createElement('input');
    enabled.type = 'checkbox';
    enabled.checked = rule.enabled !== false;
    enabled.title = 'Regla activa';
    enabled.addEventListener('change', () => { rule.enabled = enabled.checked; });
    const copy = document.createElement('button');
    copy.type = 'button';
    copy.className = 'analytics-rule-copy';
    const strong = document.createElement('strong');
    const small = document.createElement('small');
    strong.textContent = rule.name || 'Regla';
    const target = rule.zone_id
      ? state.analyticsConfig.zones?.find(zone => zone.id === rule.zone_id)?.name
      : state.analyticsConfig.lines?.find(line => line.id === rule.line_id)?.name;
    small.textContent = `${rule.class_name === '*' ? 'Cualquier objeto' : analyticsClassLabel(rule.class_name)} · ${target || rule.event_type}`;
    copy.append(strong, small);
    copy.addEventListener('click', () => editAnalyticsRule(index));
    const remove = document.createElement('button');
    remove.type = 'button'; remove.className = 'icon-button'; remove.textContent = '×'; remove.title = 'Eliminar regla';
    remove.addEventListener('click', () => { rules.splice(index, 1); renderAnalyticsRules(); resetAnalyticsRuleEditor(); });
    row.append(enabled, copy, remove);
    list.append(row);
  });
  if (!rules.length) {
    const empty = document.createElement('p'); empty.className = 'hint analytics-inline-empty'; empty.textContent = 'Sin reglas configuradas.'; list.append(empty);
  }
}

function saveAnalyticsRuleDraft() {
  if (!state.analyticsConfig) return;
  const name = $('#analyticsRuleName').value.trim();
  const className = $('#analyticsRuleClass').value;
  const eventType = $('#analyticsRuleType').value;
  const start = $('#analyticsRuleStart').value;
  const end = $('#analyticsRuleEnd').value;
  const cooldown = Number($('#analyticsRuleCooldown').value) || 30;
  const errorBox = $('#analyticsConfigError');
  errorBox.textContent = '';
  if (!name) { errorBox.textContent = 'Escribe un nombre para la regla.'; return; }
  if ((start && !end) || (!start && end)) { errorBox.textContent = 'Completa ambas horas o deja ambas vacías.'; return; }
  const zoneID = eventType === 'zone_enter' || eventType === 'zone_exit' ? $('#analyticsRuleZone').value : '';
  const lineID = eventType === 'line_cross' ? $('#analyticsRuleLine').value : '';
  if ((eventType === 'zone_enter' || eventType === 'zone_exit') && !zoneID) { errorBox.textContent = 'Crea y selecciona una zona para esta regla.'; return; }
  if (eventType === 'line_cross' && !lineID) { errorBox.textContent = 'Crea y selecciona una línea para esta regla.'; return; }
  const days = $$('#analyticsRuleDays input[type="checkbox"]:checked').map(input => Number(input.value));
  const existing = state.analyticsRuleEditIndex >= 0 ? state.analyticsConfig.rules[state.analyticsRuleEditIndex] : null;
  const rule = {
    id: existing?.id || analyticsClientID('r'),
    name,
    enabled: existing?.enabled !== false,
    class_name: className,
    event_type: eventType,
    zone_id: zoneID,
    line_id: lineID,
    start,
    end,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'Local',
    days,
    telegram: $('#analyticsRuleTelegram').checked,
    cooldown_seconds: Math.max(1, Math.min(3600, cooldown)),
  };
  state.analyticsConfig.rules ||= [];
  if (state.analyticsRuleEditIndex >= 0) state.analyticsConfig.rules[state.analyticsRuleEditIndex] = rule;
  else state.analyticsConfig.rules.push(rule);
  renderAnalyticsRules();
  resetAnalyticsRuleEditor();
}

async function saveAnalyticsConfig(event) {
  event.preventDefault();
  if (!state.analyticsConfig || !state.analyticsCameraID) return;
  if (state.analyticsDrawing) {
    $('#analyticsConfigError').textContent = 'Termina o cancela el dibujo antes de guardar.';
    return;
  }
  if (!(state.analyticsConfig.enabled_classes || []).length) {
    $('#analyticsConfigError').textContent = 'Selecciona al menos una clase para detectar.';
    return;
  }
  const unnamedGeometry = [...(state.analyticsConfig.zones || []), ...(state.analyticsConfig.lines || [])].some(item => !String(item.name || '').trim());
  if (unnamedGeometry) {
    $('#analyticsConfigError').textContent = 'Todas las zonas y líneas necesitan un nombre.';
    return;
  }
  const button = $('#analyticsSaveBtn');
  button.disabled = true;
  $('#analyticsConfigError').textContent = '';
  $('#analyticsConfigMessage').textContent = 'Guardando…';
  try {
    const saved = await api(`/api/analytics/config/${encodeURIComponent(state.analyticsCameraID)}`, {
      method: 'PUT', body: JSON.stringify(state.analyticsConfig),
    });
    state.analyticsConfig = cloneAnalyticsConfig(saved);
    $('#analyticsConfigMessage').textContent = 'Guardado';
    $('#analyticsDialog').close('saved');
    setStatus('Analítica inteligente actualizada; el histórico de esta cámara se reanalizará.', true);
    if (state.recordingCameraID === state.analyticsCameraID) {
      state.objectSegments = [];
      setTimeout(() => loadRecordingAnalytics({ silent: true }), 700);
    }
  } catch (error) {
    if (!(error instanceof SessionRequiredError)) $('#analyticsConfigError').textContent = error.message;
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

$('#analyticsConfigForm').addEventListener('submit', saveAnalyticsConfig);
$('#analyticsClassSearch').addEventListener('input', renderAnalyticsClassList);
$('#analyticsDrawZoneBtn').addEventListener('click', () => startAnalyticsDrawing('zone'));
$('#analyticsDrawLineBtn').addEventListener('click', () => startAnalyticsDrawing('line'));
$('#analyticsFinishDrawBtn').addEventListener('click', finishAnalyticsDrawing);
$('#analyticsCancelDrawBtn').addEventListener('click', cancelAnalyticsDrawing);
$('#analyticsOverlay').addEventListener('click', handleAnalyticsOverlayPoint);
$('#analyticsRuleType').addEventListener('change', syncAnalyticsRuleTypeFields);
$('#analyticsRuleAddBtn').addEventListener('click', saveAnalyticsRuleDraft);
$('#analyticsRuleResetBtn').addEventListener('click', resetAnalyticsRuleEditor);
$('#analyticsDialog').addEventListener('close', () => {
  state.analyticsDrawing = null;
  state.analyticsCameraID = '';
  $('#analyticsPreviewImage').removeAttribute('src');
});

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
$('#recordingDeleteBtn').addEventListener('click', deleteCurrentRecording);
$('#recordingsDeleteFilteredBtn').addEventListener('click', deleteFilteredRecordings);
$('#recordingsDeleteAllBtn').addEventListener('click', deleteAllRecordings);
$('#recordingDeleteScope').addEventListener('change', syncRecordingDeleteScope);
$('#recordingDeleteFrom').addEventListener('change', updateRecordingDeleteAction);
$('#recordingDeleteTo').addEventListener('change', updateRecordingDeleteAction);
$('#timelineZoomOutBtn').addEventListener('click', () => setTimelineZoom(state.timelineZoom / 1.5));
$('#timelineZoomInBtn').addEventListener('click', () => setTimelineZoom(state.timelineZoom * 1.5));
$('#timelineZoomResetBtn').addEventListener('click', () => setTimelineZoom(TIMELINE_ZOOM_MIN));
const timelineViewport = $('#timelineViewport');
timelineViewport.addEventListener('wheel', (event) => {
  if (!event.ctrlKey && !event.metaKey) return;
  event.preventDefault();
  const factor = Math.exp(-event.deltaY * .0025);
  setTimelineZoom(state.timelineZoom * factor, event.clientX);
}, { passive: false });
timelineViewport.addEventListener('pointerdown', handleTimelinePointerDown);
timelineViewport.addEventListener('pointermove', handleTimelinePointerMove);
timelineViewport.addEventListener('pointerup', handleTimelinePointerEnd);
timelineViewport.addEventListener('pointercancel', handleTimelinePointerEnd);
timelineViewport.addEventListener('click', (event) => {
  if (Date.now() >= timelineSuppressClickUntil) return;
  event.preventDefault();
  event.stopPropagation();
}, true);
timelineViewport.addEventListener('keydown', (event) => {
  if (event.key === '+' || event.key === '=') {
    event.preventDefault();
    setTimelineZoom(state.timelineZoom * 1.5);
  } else if (event.key === '-' || event.key === '_') {
    event.preventDefault();
    setTimelineZoom(state.timelineZoom / 1.5);
  } else if (event.key === '0') {
    event.preventDefault();
    setTimelineZoom(TIMELINE_ZOOM_MIN);
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault();
    const direction = event.key === 'ArrowLeft' ? -1 : 1;
    timelineViewport.scrollBy({ left: direction * Math.max(80, timelineViewport.clientWidth * .2), behavior: 'smooth' });
  }
});
$('#recordingDate').addEventListener('change', async (event) => {
  state.recordingDate = event.target.value;
  state.analyticsSummary = null;
  state.analyticsSummaryKey = '';
  state.analyticsSummaryFetchedAt = 0;
  await loadRecordingPage(1);
  updateRecordingDeleteAction();
});
$('#recordingPagePrev').addEventListener('click', () => {
  if (state.recordingPage.page > 1) loadRecordingPage(state.recordingPage.page - 1);
});
$('#recordingPageNext').addEventListener('click', () => {
  if (state.recordingPage.page < state.recordingPage.total_pages) loadRecordingPage(state.recordingPage.page + 1);
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
$('#recordingClosePlayer').addEventListener('click', backToRecordingLibrary);
$('#playbackErrorBackBtn').addEventListener('click', backToRecordingLibrary);

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
