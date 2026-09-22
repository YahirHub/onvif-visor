package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"onvif-visor/internal/media"
)

type recordingDeleteBatchRequest struct {
	CameraID       string `json:"camera_id"`
	All            bool   `json:"all"`
	WithoutPersons bool   `json:"without_persons,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
}

type recordingDeleteBatchResponse struct {
	Deleted           []string `json:"deleted"`
	DeletedCount      int      `json:"deleted_count"`
	SkippedActive     int      `json:"skipped_active"`
	SkippedPersons    int      `json:"skipped_persons,omitempty"`
	SkippedUnanalyzed int      `json:"skipped_unanalyzed,omitempty"`
}

func (a *App) forgetRecordingEvents(cameraID string, names []string) {
	if len(names) == 0 {
		return
	}
	deleted := make(map[string]struct{}, len(names))
	for _, name := range names {
		deleted[name] = struct{}{}
	}
	a.eventsMu.Lock()
	kept := a.events[:0]
	for _, event := range a.events {
		if event.CameraID == cameraID {
			if _, remove := deleted[event.RecordingName]; remove {
				continue
			}
		}
		kept = append(kept, event)
	}
	a.events = kept
	a.eventsMu.Unlock()
}

func (a *App) deleteRecordingArtifacts(ctx context.Context, cameraID, name string) error {
	if err := a.media.DeleteRecording(cameraID, name); err != nil {
		return err
	}
	if a.analytics != nil {
		_ = a.analytics.ForgetRecording(ctx, cameraID, name)
	}
	a.forgetRecordingEvents(cameraID, []string{name})
	return nil
}

func (a *App) handleRecordingMetadata(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	metadata, err := a.media.RecordingMetadata(ctx, cameraID, name)
	if err != nil {
		if errors.Is(err, media.ErrRecordingNotFound) {
			writeError(w, http.StatusNotFound, "Grabación no encontrada.")
			return
		}
		writeError(w, http.StatusInternalServerError, "No se pudo consultar la duración real de la grabación.")
		return
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (a *App) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	if err := a.deleteRecordingArtifacts(r.Context(), cameraID, name); err != nil {
		switch {
		case errors.Is(err, media.ErrRecordingNotFound):
			writeError(w, http.StatusNotFound, "Grabación no encontrada.")
		case errors.Is(err, media.ErrRecordingActive):
			writeError(w, http.StatusConflict, "No se puede eliminar una grabación que todavía está activa.")
		default:
			writeError(w, http.StatusInternalServerError, "No se pudo eliminar la grabación.")
		}
		return
	}
	if session, ok := currentSession(r); ok {
		log.Printf("recording_delete user=%s camera=%s name=%s", session.Username, cameraID, name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "name": name})
}

func (a *App) handleDeleteRecordingsBatch(w http.ResponseWriter, r *http.Request) {
	var req recordingDeleteBatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Filtro de eliminación inválido.")
		return
	}
	req.CameraID = strings.TrimSpace(req.CameraID)
	if req.CameraID == "" {
		writeError(w, http.StatusBadRequest, "Selecciona una cámara.")
		return
	}
	if _, err := a.store.Get(req.CameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}

	var from, to time.Time
	if req.All {
		if strings.TrimSpace(req.From) != "" || strings.TrimSpace(req.To) != "" {
			writeError(w, http.StatusBadRequest, "No combines all con un rango de fechas.")
			return
		}
	} else {
		var err error
		from, err = time.Parse(time.RFC3339, strings.TrimSpace(req.From))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Fecha inicial inválida.")
			return
		}
		to, err = time.Parse(time.RFC3339, strings.TrimSpace(req.To))
		if err != nil || !to.After(from) {
			writeError(w, http.StatusBadRequest, "Fecha final inválida.")
			return
		}
	}

	files, err := a.media.ListRecordings(req.CameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar las grabaciones.")
		return
	}

	var analyzed, persons map[string]bool
	if req.WithoutPersons {
		if a.analytics == nil {
			writeError(w, http.StatusServiceUnavailable, "La analítica debe estar disponible para eliminar videos sin personas.")
			return
		}
		analyzed, persons, err = a.analytics.RecordingPersonStates(r.Context(), req.CameraID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudo comprobar qué videos contienen personas.")
			return
		}
	}
	result := recordingDeleteBatchResponse{Deleted: make([]string, 0)}
	for _, file := range files {
		if !req.All && (file.StartedAt.Before(from) || !file.StartedAt.Before(to)) {
			continue
		}
		if file.Active {
			result.SkippedActive++
			continue
		}
		if req.WithoutPersons {
			if !analyzed[file.Name] {
				result.SkippedUnanalyzed++
				continue
			}
			if persons[file.Name] {
				result.SkippedPersons++
				continue
			}
		}
		if err := a.deleteRecordingArtifacts(r.Context(), req.CameraID, file.Name); err != nil {
			if errors.Is(err, media.ErrRecordingActive) {
				result.SkippedActive++
				continue
			}
			if errors.Is(err, media.ErrRecordingNotFound) {
				continue
			}
			writeError(w, http.StatusInternalServerError, "La eliminación por lote quedó incompleta.")
			return
		}
		result.Deleted = append(result.Deleted, file.Name)
	}
	result.DeletedCount = len(result.Deleted)
	if session, ok := currentSession(r); ok {
		log.Printf("recording_delete_batch user=%s camera=%s deleted=%d skipped_active=%d skipped_persons=%d skipped_unanalyzed=%d all=%t without_persons=%t", session.Username, req.CameraID, result.DeletedCount, result.SkippedActive, result.SkippedPersons, result.SkippedUnanalyzed, req.All, req.WithoutPersons)
	}
	writeJSON(w, http.StatusOK, result)
}
