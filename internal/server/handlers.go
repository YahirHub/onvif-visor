package server

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"onvif-visor/internal/analytics"
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
		a.syncAnalytics(full)
	}
	cam.Password = ""
	writeJSON(w, http.StatusCreated, cam)
}

func (a *App) handleDeleteCamera(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.stopWatcher(id)
	if a.analytics != nil {
		a.analytics.RemoveCamera(id)
	}
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

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	a.eventsMu.Lock()
	out := append([]CameraEvent(nil), a.events...)
	a.eventsMu.Unlock()

	bestByCamera := make(map[string]map[string]analytics.PersonSegment)
	missing := make(map[string]map[string]struct{})
	filtered := out[:0]
	for _, event := range out {
		if event.RecordingName != "" && !a.media.RecordingExists(event.CameraID, event.RecordingName) {
			if missing[event.CameraID] == nil {
				missing[event.CameraID] = make(map[string]struct{})
			}
			missing[event.CameraID][event.RecordingName] = struct{}{}
			continue
		}
		if event.RecordingName != "" && a.analytics != nil {
			best, ok := bestByCamera[event.CameraID]
			if !ok {
				best = make(map[string]analytics.PersonSegment)
				segments, err := a.analytics.Segments(r.Context(), event.CameraID)
				if err == nil {
					for _, segment := range segments {
						current, exists := best[segment.RecordingName]
						segmentScore := segment.BestScore
						if segmentScore <= 0 {
							segmentScore = segment.MaxConfidence
						}
						currentScore := current.BestScore
						if currentScore <= 0 {
							currentScore = current.MaxConfidence
						}
						if !exists || segmentScore > currentScore {
							best[segment.RecordingName] = segment
						}
					}
				}
				bestByCamera[event.CameraID] = best
			}
			if segment, ok := best[event.RecordingName]; ok {
				event.PersonOffset = segment.PeakSecond
				event.PersonConfidence = segment.MaxConfidence
			}
		}
		filtered = append(filtered, event)
	}
	if len(missing) > 0 {
		a.eventsMu.Lock()
		kept := a.events[:0]
		for _, event := range a.events {
			if names := missing[event.CameraID]; names != nil {
				if _, remove := names[event.RecordingName]; remove {
					continue
				}
			}
			kept = append(kept, event)
		}
		a.events = kept
		a.eventsMu.Unlock()
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (a *App) handleRecordingStorage(w http.ResponseWriter, _ *http.Request) {
	stats, err := a.media.RecordingStorageStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo calcular el almacenamiento de grabaciones")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

const (
	defaultRecordingPageSize = 24
	maxRecordingPageSize     = 100
	maxRecordingTimelineSpan = 48 * time.Hour
)

type recordingCameraSummary struct {
	CameraID       string     `json:"camera_id"`
	Count          int        `json:"count"`
	SizeBytes      int64      `json:"size_bytes"`
	DurationSecond float64    `json:"duration_seconds"`
	LatestStarted  *time.Time `json:"latest_started_at,omitempty"`
}

type recordingLibrarySummary struct {
	Cameras    []recordingCameraSummary `json:"cameras"`
	TotalBytes int64                    `json:"total_bytes"`
	FileCount  int                      `json:"file_count"`
}

type recordingPageResponse struct {
	Items      []media.RecordingFile `json:"items"`
	Timeline   []media.RecordingFile `json:"timeline"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	Total      int                   `json:"total"`
	TotalPages int                   `json:"total_pages"`
}

func summarizeRecordings(cameraID string, files []media.RecordingFile) recordingCameraSummary {
	summary := recordingCameraSummary{CameraID: cameraID}
	for i := range files {
		file := files[i]
		summary.Count++
		summary.SizeBytes += file.SizeBytes
		summary.DurationSecond += file.DurationSecond
		if summary.LatestStarted == nil || file.StartedAt.After(*summary.LatestStarted) {
			started := file.StartedAt
			summary.LatestStarted = &started
		}
	}
	return summary
}

func recordingsInRange(files []media.RecordingFile, from, to time.Time) []media.RecordingFile {
	if from.IsZero() || to.IsZero() {
		return append([]media.RecordingFile(nil), files...)
	}
	filtered := make([]media.RecordingFile, 0, len(files))
	for _, file := range files {
		duration := time.Duration(max(file.DurationSecond, 1) * float64(time.Second))
		end := file.StartedAt.Add(duration)
		if end.After(from) && file.StartedAt.Before(to) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func parseRecordingPageQuery(r *http.Request) (page, pageSize int, from, to time.Time, err error) {
	page = 1
	pageSize = defaultRecordingPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, time.Time{}, time.Time{}, errors.New("página inválida")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > maxRecordingPageSize {
			return 0, 0, time.Time{}, time.Time{}, errors.New("tamaño de página inválido")
		}
	}

	fromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	toRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromRaw == "" && toRaw == "" {
		return page, pageSize, time.Time{}, time.Time{}, nil
	}
	if fromRaw == "" || toRaw == "" {
		return 0, 0, time.Time{}, time.Time{}, errors.New("rango de fechas incompleto")
	}
	from, err = time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, errors.New("fecha inicial inválida")
	}
	to, err = time.Parse(time.RFC3339, toRaw)
	if err != nil || !to.After(from) {
		return 0, 0, time.Time{}, time.Time{}, errors.New("fecha final inválida")
	}
	if to.Sub(from) > maxRecordingTimelineSpan {
		return 0, 0, time.Time{}, time.Time{}, errors.New("el rango para la línea de tiempo no puede superar 48 horas")
	}
	return page, pageSize, from, to, nil
}

func paginateRecordings(files []media.RecordingFile, page, pageSize int) ([]media.RecordingFile, int, int) {
	total := len(files)
	if total == 0 {
		return []media.RecordingFile{}, 1, 0
	}
	totalPages := (total + pageSize - 1) / pageSize
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	end := min(start+pageSize, total)
	return append([]media.RecordingFile(nil), files[start:end]...), page, totalPages
}

func (a *App) handleRecordingSummary(w http.ResponseWriter, _ *http.Request) {
	storage, err := a.media.RecordingStorageStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo calcular el almacenamiento de grabaciones")
		return
	}
	cameras := a.store.List()
	out := recordingLibrarySummary{
		Cameras:    make([]recordingCameraSummary, 0, len(cameras)),
		TotalBytes: storage.TotalBytes,
		FileCount:  storage.FileCount,
	}
	for _, camera := range cameras {
		files, err := a.media.ListRecordings(camera.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "no se pudo consultar el almacenamiento de grabaciones")
			return
		}
		out.Cameras = append(out.Cameras, summarizeRecordings(camera.ID, files))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleRecordings(w http.ResponseWriter, r *http.Request) {
	cameraID := strings.TrimSpace(r.URL.Query().Get("camera_id"))
	if cameraID == "" {
		writeError(w, http.StatusBadRequest, "selecciona una cámara")
		return
	}
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "cámara no encontrada")
		return
	}
	page, pageSize, from, to, err := parseRecordingPageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	files, err := a.media.ListRecordings(cameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo consultar el almacenamiento de grabaciones")
		return
	}
	filtered := recordingsInRange(files, from, to)
	items, page, totalPages := paginateRecordings(filtered, page, pageSize)
	response := recordingPageResponse{
		Items:      items,
		Timeline:   []media.RecordingFile{},
		Page:       page,
		PageSize:   pageSize,
		Total:      len(filtered),
		TotalPages: totalPages,
	}
	if !from.IsZero() && !to.IsZero() {
		response.Timeline = filtered
	}
	writeJSON(w, http.StatusOK, response)
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
