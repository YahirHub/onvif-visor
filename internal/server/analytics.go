package server

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"onvif-visor/internal/analytics"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

func (a *App) handleSmartAnalyticsEvent(event analytics.SmartEvent) {
	cam, err := a.store.Get(event.CameraID)
	if err != nil {
		return
	}
	data := map[string]string{
		"Class":      event.ClassName,
		"Confidence": fmt.Sprintf("%.0f%%", event.Confidence*100),
		"Type":       event.Type,
	}
	if event.ZoneName != "" {
		data["Zone"] = event.ZoneName
	}
	if event.LineName != "" {
		data["Line"] = event.LineName
	}
	if event.Direction != "" {
		data["Direction"] = event.Direction
	}
	if len(event.MatchedRuleNames) > 0 {
		data["Rules"] = strings.Join(event.MatchedRuleNames, ", ")
	}
	item := CameraEvent{
		CameraID: cam.ID, CameraName: cam.Name,
		Topic: "analytics/" + event.Type, Time: event.OccurredAt,
		Data: data, RecordingName: event.RecordingName, RecordingOffset: event.OffsetSecond,
		ObjectClass: event.ClassName, ObjectOffset: event.OffsetSecond, ObjectConfidence: event.Confidence,
		SmartType: event.Type, ZoneName: event.ZoneName, LineName: event.LineName, Direction: event.Direction,
	}
	if event.ClassName == "person" {
		item.PersonOffset = event.OffsetSecond
		item.PersonConfidence = event.Confidence
	}
	a.eventsMu.Lock()
	a.events = append([]CameraEvent{item}, a.events...)
	if len(a.events) > 200 {
		a.events = a.events[:200]
	}
	a.eventsMu.Unlock()
	if event.NotifyTelegram {
		go a.notifyTelegramSmartEvent(cam, event)
	}
}

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

func (a *App) handleAnalyticsClasses(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, analytics.Classes())
}

func (a *App) handleAnalyticsConfig(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeError(w, http.StatusServiceUnavailable, "La analítica no está disponible.")
		return
	}
	cameraID := strings.TrimSpace(r.PathValue("cameraID"))
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	cfg, err := a.analytics.CameraConfig(r.Context(), cameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo cargar la configuración de analítica.")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *App) handleAnalyticsConfigSave(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeError(w, http.StatusServiceUnavailable, "La analítica no está disponible.")
		return
	}
	cameraID := strings.TrimSpace(r.PathValue("cameraID"))
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	var cfg analytics.CameraAnalyticsConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "Configuración de analítica inválida.")
		return
	}
	saved, err := a.analytics.SaveCameraConfig(r.Context(), cameraID, cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.analytics.ReindexCamera(cameraID)
	writeJSON(w, http.StatusOK, saved)
}

func (a *App) handleAnalyticsObjects(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeJSON(w, http.StatusOK, []analytics.ObjectSegment{})
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
	segments, err := a.analytics.ObjectSegments(r.Context(), cameraID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar las detecciones de objetos.")
		return
	}
	writeJSON(w, http.StatusOK, segments)
}

func (a *App) handleAnalyticsEvents(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeJSON(w, http.StatusOK, []analytics.SmartEvent{})
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
	from, to, err := parseAnalyticsRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	events, err := a.analytics.SmartEvents(r.Context(), cameraID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar los eventos inteligentes.")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func parseAnalyticsRange(r *http.Request) (time.Time, time.Time, error) {
	var from, to time.Time
	for key, target := range map[string]*time.Time{"from": &from, "to": &to} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("fecha %s inválida", key)
		}
		*target = parsed
	}
	if !from.IsZero() && !to.IsZero() && !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("el rango de fechas es inválido")
	}
	return from, to, nil
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

type analyticsSummaryClass struct {
	ClassName string `json:"class_name"`
	Label     string `json:"label"`
	Count     int    `json:"count"`
}

type analyticsDailySummary struct {
	Date          string                  `json:"date"`
	CameraID      string                  `json:"camera_id"`
	TotalObjects  int                     `json:"total_objects"`
	Classes       []analyticsSummaryClass `json:"classes"`
	BusiestHour   int                     `json:"busiest_hour"`
	BusiestCount  int                     `json:"busiest_count"`
	ZoneEntries   int                     `json:"zone_entries"`
	ZoneExits     int                     `json:"zone_exits"`
	LineCrossings int                     `json:"line_crossings"`
	RuleMatches   int                     `json:"rule_matches"`
}

func (a *App) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if a.analytics == nil {
		writeError(w, http.StatusServiceUnavailable, "La analítica no está disponible.")
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
	location, err := analyticsRequestLocation(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		date = time.Now().In(location).Format("2006-01-02")
	}
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Fecha de resumen inválida.")
		return
	}
	end := start.AddDate(0, 0, 1)

	events, err := a.analytics.SmartEvents(r.Context(), cameraID, start.UTC(), end.UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar los eventos del resumen.")
		return
	}
	counts := make(map[string]int)
	hours := make([]int, 24)
	summary := analyticsDailySummary{Date: date, CameraID: cameraID}
	for _, event := range events {
		switch event.Type {
		case "object":
			summary.TotalObjects++
			counts[event.ClassName]++
			hours[event.OccurredAt.In(location).Hour()]++
		case "zone_enter":
			summary.ZoneEntries++
		case "zone_exit":
			summary.ZoneExits++
		case "line_cross":
			summary.LineCrossings++
		}
		if len(event.MatchedRuleIDs) > 0 {
			summary.RuleMatches++
		}
	}
	labels := make(map[string]string)
	for _, class := range analytics.Classes() {
		labels[class.Name] = class.Label
	}
	summary.Classes = make([]analyticsSummaryClass, 0, len(counts))
	for name, count := range counts {
		summary.Classes = append(summary.Classes, analyticsSummaryClass{ClassName: name, Label: labels[name], Count: count})
	}
	sort.Slice(summary.Classes, func(i, j int) bool {
		if summary.Classes[i].Count == summary.Classes[j].Count {
			return summary.Classes[i].Label < summary.Classes[j].Label
		}
		return summary.Classes[i].Count > summary.Classes[j].Count
	})
	for hour, count := range hours {
		if count > summary.BusiestCount {
			summary.BusiestHour, summary.BusiestCount = hour, count
		}
	}
	writeJSON(w, http.StatusOK, summary)
}

func analyticsRequestLocation(r *http.Request) (*time.Location, error) {
	if name := strings.TrimSpace(r.URL.Query().Get("timezone")); name != "" {
		location, err := time.LoadLocation(name)
		if err != nil {
			return nil, fmt.Errorf("zona horaria inválida")
		}
		return location, nil
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset_minutes")); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < -840 || offset > 840 {
			return nil, fmt.Errorf("offset de zona horaria inválido")
		}
		return time.FixedZone("browser", -offset*60), nil
	}
	return time.Local, nil
}

func (a *App) handleAnalyticsReindex(w http.ResponseWriter, _ *http.Request) {
	if a.analytics == nil || !a.analytics.Status().Ready {
		writeError(w, http.StatusServiceUnavailable, "La analítica NanoDet no está disponible.")
		return
	}
	a.analytics.Reindex()
	writeJSON(w, http.StatusAccepted, map[string]bool{"queued": true})
}
