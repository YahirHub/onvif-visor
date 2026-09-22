package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"onvif-visor/internal/analytics"
	"onvif-visor/internal/media"
	"onvif-visor/internal/store"
)

func TestDeleteRecordingsBatchUsesRequestedRange(t *testing.T) {
	dataDir := t.TempDir()
	cameraStore, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cam, err := cameraStore.Add(store.Camera{ID: "cam-1", Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	recordingsDir := t.TempDir()
	manager, err := media.New("ffmpeg", recordingsDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(recordingsDir, cam.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	inRange := "20260815-120000.000.mkv"
	outside := "20260915-120000.000.mkv"
	for _, name := range []string{inRange, outside} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{store: cameraStore, media: manager}
	body := `{"camera_id":"cam-1","from":"2026-08-01T00:00:00Z","to":"2026-09-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recordings/delete-batch", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleDeleteRecordingsBatch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, inRange)); !os.IsNotExist(err) {
		t.Fatalf("in-range recording still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, outside)); err != nil {
		t.Fatalf("outside recording removed: %v", err)
	}
	if !strings.Contains(rec.Body.String(), `"deleted_count":1`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestDeleteRecordingsBatchRejectsAllWithRange(t *testing.T) {
	cameraStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cameraStore.Add(store.Camera{ID: "cam-1", Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	manager, err := media.New("ffmpeg", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: cameraStore, media: manager}
	body := `{"camera_id":"cam-1","all":true,"from":"2026-08-01T00:00:00Z","to":"2026-09-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/recordings/delete-batch", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleDeleteRecordingsBatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRecordingsBatchWithoutPersonsKeepsPersonAndPendingClips(t *testing.T) {
	dataDir := t.TempDir()
	cameraStore, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cam, err := cameraStore.Add(store.Camera{ID: "cam-1", Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}

	recordingsDir := t.TempDir()
	mediaManager, err := media.New("ffmpeg", recordingsDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(recordingsDir, cam.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	withPerson := "20260815-120000.000.mkv"
	withoutPerson := "20260815-121000.000.mkv"
	pending := "20260815-122000.000.mkv"
	for _, name := range []string{withPerson, withoutPerson, pending} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	analyticsDir := t.TempDir()
	analyticsManager, err := analytics.New(analytics.Config{
		DataDir: analyticsDir, FFmpeg: "ffmpeg",
		ModelPath: filepath.Join(analyticsDir, "missing-model.onnx"), Media: mediaManager,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = analyticsManager.Close() })

	analyticsStore, err := analytics.OpenStore(analyticsDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := analyticsStore.CompleteObjects(context.Background(), analytics.RecordingAnalysis{
		CameraID: cam.ID, RecordingName: withPerson, SizeBytes: 5, ModifiedUnixNS: 1,
		DurationSecond: 10, Model: analytics.AnalysisModelKey, AnalyzedAt: now,
	}, []analytics.ObjectSegment{{ClassID: 0, ClassName: "person", StartSecond: 1, EndSecond: 3, PeakSecond: 2, MaxConfidence: .8, BestScore: .9, Source: "offline"}}, nil); err != nil {
		_ = analyticsStore.Close()
		t.Fatal(err)
	}
	if err := analyticsStore.CompleteObjects(context.Background(), analytics.RecordingAnalysis{
		CameraID: cam.ID, RecordingName: withoutPerson, SizeBytes: 5, ModifiedUnixNS: 2,
		DurationSecond: 10, Model: analytics.AnalysisModelKey, AnalyzedAt: now,
	}, nil, nil); err != nil {
		_ = analyticsStore.Close()
		t.Fatal(err)
	}
	if err := analyticsStore.MarkAnalyzing(context.Background(), cam.ID, pending, 5, 3, 10, analytics.AnalysisModelKey); err != nil {
		_ = analyticsStore.Close()
		t.Fatal(err)
	}
	if err := analyticsStore.Close(); err != nil {
		t.Fatal(err)
	}

	app := &App{store: cameraStore, media: mediaManager, analytics: analyticsManager}
	body := "{\"camera_id\":\"cam-1\",\"all\":true,\"without_persons\":true}"
	req := httptest.NewRequest(http.MethodPost, "/api/recordings/delete-batch", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleDeleteRecordingsBatch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, withoutPerson)); !os.IsNotExist(err) {
		t.Fatalf("recording without persons still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, withPerson)); err != nil {
		t.Fatalf("recording with person was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, pending)); err != nil {
		t.Fatalf("pending recording was removed: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "\"deleted_count\":1") ||
		!strings.Contains(rec.Body.String(), "\"skipped_persons\":1") ||
		!strings.Contains(rec.Body.String(), "\"skipped_unanalyzed\":1") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}
func TestForgetRecordingEventsRemovesOnlyDeletedClips(t *testing.T) {
	app := &App{events: []CameraEvent{
		{CameraID: "cam-1", RecordingName: "deleted.mkv"},
		{CameraID: "cam-1", RecordingName: "kept.mkv"},
		{CameraID: "cam-2", RecordingName: "deleted.mkv"},
	}}
	app.forgetRecordingEvents("cam-1", []string{"deleted.mkv"})
	if len(app.events) != 2 {
		t.Fatalf("events=%#v", app.events)
	}
	for _, event := range app.events {
		if event.CameraID == "cam-1" && event.RecordingName == "deleted.mkv" {
			t.Fatal("deleted recording event is still present")
		}
	}
}

func TestHandleEventsDropsMissingRecordings(t *testing.T) {
	manager, err := media.New("ffmpeg", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{media: manager, events: []CameraEvent{
		{CameraID: "cam-1", CameraName: "Test", RecordingName: "missing.mkv"},
		{CameraID: "cam-1", CameraName: "Test", Topic: "event-without-recording"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	app.handleEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "missing.mkv") {
		t.Fatalf("missing recording event leaked: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event-without-recording") {
		t.Fatalf("unrecorded event was removed unexpectedly: %s", rec.Body.String())
	}
	if len(app.events) != 1 {
		t.Fatalf("events were not pruned in memory: %#v", app.events)
	}
}
