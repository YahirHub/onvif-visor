package server

import (
	"errors"
	"net/http"
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
	if req.Enabled && !cam.EventsSupported {
		_, _ = a.store.SetRecording(id, false, cam.PostRollSeconds)
		writeError(w, http.StatusConflict, "la cámara no reporta soporte de eventos ONVIF")
		return
	}
	if full, getErr := a.store.Get(id); getErr == nil {
		a.syncWatcher(full)
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

func (a *App) handleEvents(w http.ResponseWriter, _ *http.Request) {
	a.eventsMu.Lock()
	out := append([]CameraEvent(nil), a.events...)
	a.eventsMu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleRecordings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.media.AllStatus())
}

func triggerDuration(cam store.Camera) time.Duration {
	seconds := cam.PostRollSeconds
	if seconds <= 0 {
		seconds = 20
	}
	return time.Duration(seconds) * time.Second
}
