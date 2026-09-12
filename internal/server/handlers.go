package server

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"onvif-visor/internal/media"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

func (a *App) handleDiscover(w http.ResponseWriter, r *http.Request) {
	devices, err := onvifsvc.Discover(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "no fue posible completar el descubrimiento ONVIF")
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (a *App) handleListCameras(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.store.List())
}

func (a *App) handleAddCamera(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		writeError(w, http.StatusBadRequest, "endpoint requerido")
		return
	}

	probe, err := onvifsvc.Probe(r.Context(), onvifsvc.Credentials{
		Endpoint: req.Endpoint, Username: req.Username, Password: req.Password,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "no se pudo validar la cámara ONVIF; revise endpoint y credenciales")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(strings.Join([]string{probe.Manufacturer, probe.Model}, " "))
	}
	if name == "" {
		name = "Cámara ONVIF"
	}
	cam, err := a.store.Add(store.Camera{
		Name:            name,
		Endpoint:        probe.Endpoint,
		Username:        req.Username,
		Password:        req.Password,
		Manufacturer:    probe.Manufacturer,
		Model:           probe.Model,
		FirmwareVersion: probe.FirmwareVersion,
		ProfileToken:    probe.ProfileToken,
		ProfileName:     probe.ProfileName,
		RTSPURI:         probe.RTSPURI,
		EventsSupported: probe.EventsSupported,
		PostRollSeconds: 20,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo guardar la cámara")
		return
	}
	if full, getErr := a.store.Get(cam.ID); getErr == nil {
		a.syncWatcher(full)
	}
	cam.Password = ""
	writeJSON(w, http.StatusCreated, cam)
}

func (a *App) handleDeleteCamera(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.stopWatcher(id)
	a.media.Stop(id)
	if err := a.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cámara no encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "no se pudo eliminar la cámara")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRecordingSettings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled         bool `json:"enabled"`
		PostRollSeconds int  `json:"post_roll_seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.PostRollSeconds < 0 || req.PostRollSeconds > 600 {
		writeError(w, http.StatusBadRequest, "post-roll debe estar entre 1 y 600 segundos")
		return
	}
	cam, err := a.store.SetRecording(id, req.Enabled, req.PostRollSeconds)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cámara no encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "no se pudo guardar la configuración")
		return
	}
	if full, getErr := a.store.Get(id); getErr == nil {
		a.syncWatcher(full)
	}
	writeJSON(w, http.StatusOK, cam)
}

func (a *App) handleScheduleSettings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled  bool   `json:"enabled"`
		Days     []int  `json:"days"`
		Start    string `json:"start"`
		End      string `json:"end"`
		Timezone string `json:"timezone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}

	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = "Local"
	}
	if req.Enabled {
		if _, _, ok := parseClock(start); !ok {
			writeError(w, http.StatusBadRequest, "hora de inicio inválida")
			return
		}
		if _, _, ok := parseClock(end); !ok {
			writeError(w, http.StatusBadRequest, "hora final inválida")
			return
		}
		if timezone != "Local" {
			if _, err := time.LoadLocation(timezone); err != nil {
				writeError(w, http.StatusBadRequest, "zona horaria inválida")
				return
			}
		}
	}

	daySet := make(map[int]bool, 7)
	for _, day := range req.Days {
		if day < 0 || day > 6 {
			writeError(w, http.StatusBadRequest, "día de programación inválido")
			return
		}
		daySet[day] = true
	}
	days := make([]int, 0, len(daySet))
	for day := 0; day <= 6; day++ {
		if daySet[day] {
			days = append(days, day)
		}
	}
	if req.Enabled && len(days) == 0 {
		writeError(w, http.StatusBadRequest, "selecciona al menos un día")
		return
	}

	cam, err := a.store.SetSchedule(id, req.Enabled, days, start, end, timezone)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cámara no encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "no se pudo guardar la programación")
		return
	}
	writeJSON(w, http.StatusOK, cam)
}

func (a *App) handleLive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, err := a.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		writeError(w, http.StatusBadGateway, "stream RTSP inválido")
		return
	}
	if err := a.media.Live(w, r, cam.ID, streamURI); err != nil {
		if errors.Is(err, media.ErrTooManyViewers) {
			writeError(w, http.StatusTooManyRequests, "demasiados espectadores simultáneos")
			return
		}
		if r.Context().Err() == nil {
			logCameraError(cam.ID, "live", errors.New("ffmpeg live stream ended"))
		}
	}
}

func (a *App) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cam, err := a.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		writeError(w, http.StatusBadGateway, "stream RTSP inválido")
		return
	}
	image, err := a.media.Snapshot(r.Context(), streamURI)
	if err != nil {
		writeError(w, http.StatusBadGateway, "no se pudo obtener una vista previa de la cámara")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=10")
	w.Header().Set("Content-Length", strconv.Itoa(len(image)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image)
}

func (a *App) handleEvents(w http.ResponseWriter, _ *http.Request) {
	a.eventsMu.Lock()
	out := append([]CameraEvent(nil), a.events...)
	a.eventsMu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleRecordings(w http.ResponseWriter, r *http.Request) {
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	cameras := a.store.List()
	out := make([]media.RecordingFile, 0)
	found := cameraID == ""
	for _, cam := range cameras {
		if cameraID != "" && cam.ID != cameraID {
			continue
		}
		found = true
		files, err := a.media.ListRecordings(cam.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "no se pudo consultar el almacenamiento de grabaciones")
			return
		}
		out = append(out, files...)
	}
	if !found {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleRecordingSnapshot(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	offset := 0.5
	if raw := strings.TrimSpace(r.URL.Query().Get("at")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 || parsed > 86400 {
			writeError(w, http.StatusBadRequest, "posición de miniatura inválida")
			return
		}
		offset = parsed
	}
	image, err := a.media.RecordingSnapshot(r.Context(), cameraID, name, offset)
	if err != nil {
		if errors.Is(err, media.ErrRecordingNotFound) {
			writeError(w, http.StatusNotFound, "grabación no encontrada")
			return
		}
		writeError(w, http.StatusBadGateway, "no se pudo generar la miniatura de la grabación")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("Content-Length", strconv.Itoa(len(image)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image)
}

func (a *App) handleRecordingPlayback(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	start := 0.0
	if raw := strings.TrimSpace(r.URL.Query().Get("start")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 || parsed > 86400 {
			writeError(w, http.StatusBadRequest, "posición de reproducción inválida")
			return
		}
		start = parsed
	}
	if err := a.media.Playback(w, r, cameraID, name, start); err != nil {
		switch {
		case errors.Is(err, media.ErrRecordingNotFound):
			writeError(w, http.StatusNotFound, "grabación no encontrada")
		case errors.Is(err, media.ErrTooManyPlayback):
			writeError(w, http.StatusTooManyRequests, "hay demasiadas reproducciones simultáneas")
		default:
			if r.Context().Err() == nil {
				logCameraError(cameraID, "recording-playback", errors.New("ffmpeg recording playback ended"))
			}
		}
	}
}

func (a *App) handleRecordingDownload(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	path, err := a.media.RecordingPath(cameraID, name)
	if err != nil {
		if errors.Is(err, media.ErrRecordingNotFound) {
			writeError(w, http.StatusNotFound, "grabación no encontrada")
			return
		}
		writeError(w, http.StatusInternalServerError, "no se pudo abrir la grabación")
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Type", "video/x-matroska")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeFile(w, r, path)
}

func triggerDuration(cam store.Camera) time.Duration {
	seconds := cam.PostRollSeconds
	if seconds <= 0 {
		seconds = 20
	}
	return time.Duration(seconds) * time.Second
}
