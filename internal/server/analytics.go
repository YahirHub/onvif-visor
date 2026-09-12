package server

import (
	"net/http"
	"strings"

	"onvif-visor/internal/analytics"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

func (a *App) startExistingAnalytics() {
	if a.analytics == nil {
		return
	}
	for _, view := range a.store.List() {
		cam, err := a.store.Get(view.ID)
		if err != nil {
			continue
		}
		a.syncAnalytics(cam)
	}
	a.analytics.ScanNow()
}

func (a *App) syncAnalytics(cam store.Camera) {
	if a.analytics == nil {
		return
	}
	streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
	if err != nil {
		logCameraError(cam.ID, "analytics-uri", err)
		return
	}
	a.analytics.SyncCamera(analytics.CameraSource{ID: cam.ID, StreamURI: streamURI})
}

func (a *App) handleAnalyticsStatus(w http.ResponseWriter, _ *http.Request) {
	if a.analytics == nil {
		writeJSON(w, http.StatusOK, analytics.Status{Enabled: false, Ready: false, Model: analytics.ModelName, SampleFPS: 1})
		return
	}
	writeJSON(w, http.StatusOK, a.analytics.Status())
}

func (a *App) handleAnalyticsPersons(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeJSON(w, http.StatusOK, []analytics.PersonSegment{})
		return
	}
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "Selecciona una cámara.")
		return
	}
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	segments, err := a.analytics.Segments(r.Context(), cameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar las detecciones de personas.")
		return
	}
	writeJSON(w, http.StatusOK, segments)
}

func (a *App) handleAnalyticsRecordings(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeJSON(w, http.StatusOK, []analytics.RecordingAnalysis{})
		return
	}
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "Selecciona una cámara.")
		return
	}
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	items, err := a.analytics.Analyses(r.Context(), cameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo consultar el estado de la analítica.")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleAnalyticsReindex(w http.ResponseWriter, _ *http.Request) {
	if a.analytics == nil || !a.analytics.Status().Ready {
		writeError(w, http.StatusServiceUnavailable, "La analítica de personas no está disponible.")
		return
	}
	a.analytics.Reindex()
	writeJSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}
