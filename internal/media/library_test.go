package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordingPathRejectsTraversal(t *testing.T) {
	m := &Manager{recordingsDir: t.TempDir()}
	for _, name := range []string{"../video.mkv", "sub/video.mkv", "video.mp4", ""} {
		if _, err := m.RecordingPath("camera-1", name); !errors.Is(err, ErrRecordingNotFound) {
			t.Fatalf("RecordingPath(%q) err = %v, want ErrRecordingNotFound", name, err)
		}
	}
}

func TestListRecordingsParsesGeneratedNames(t *testing.T) {
	root := t.TempDir()
	m := &Manager{recordingsDir: root, recorders: make(map[string]*recording)}
	cameraID := "camera-1"
	dir := filepath.Join(root, safePathPart(cameraID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	started := time.Date(2026, 9, 12, 12, 30, 45, 123000000, time.UTC)
	name := started.Format("20060102-150405.000") + ".mkv"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	ended := started.Add(42 * time.Second)
	if err := os.Chtimes(path, ended, ended); err != nil {
		t.Fatal(err)
	}

	files, err := m.ListRecordings(cameraID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	got := files[0]
	if got.Name != name || got.CameraID != cameraID {
		t.Fatalf("unexpected recording metadata: %#v", got)
	}
	if got.StartedAt != started {
		t.Fatalf("StartedAt = %v, want %v", got.StartedAt, started)
	}
	if got.DurationSecond < 41.9 || got.DurationSecond > 42.1 {
		t.Fatalf("DurationSecond = %f, want ~42", got.DurationSecond)
	}
}

func TestRecordingMetadataUsesExactDurationAndCache(t *testing.T) {
	root := t.TempDir()
	cameraID := "camera-1"
	dir := filepath.Join(root, safePathPart(cameraID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	name := started.Format("20060102-150405.000") + ".mkv"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("fake mkv"), 0o600); err != nil {
		t.Fatal(err)
	}
	ended := started.Add(42 * time.Second)
	if err := os.Chtimes(path, ended, ended); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(t.TempDir(), "ffprobe")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\necho 7.25\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := &Manager{recordingsDir: root, ffprobe: probe, recorders: make(map[string]*recording), durationCache: make(map[string]durationCacheEntry)}
	metadata, err := m.RecordingMetadata(context.Background(), cameraID, name)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DurationSecond != 7.25 {
		t.Fatalf("DurationSecond = %f, want 7.25", metadata.DurationSecond)
	}
	if err := os.WriteFile(probe, []byte("#!/bin/sh\necho 99\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata, err = m.RecordingMetadata(context.Background(), cameraID, name)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DurationSecond != 7.25 {
		t.Fatalf("cached DurationSecond = %f, want 7.25", metadata.DurationSecond)
	}
}

func TestDeleteRecordingRejectsActiveAndDeletesClosed(t *testing.T) {
	root := t.TempDir()
	cameraID := "camera-1"
	dir := filepath.Join(root, safePathPart(cameraID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	activeName := "20260912-120000.000.mkv"
	closedName := "20260912-120100.000.mkv"
	activePath := filepath.Join(dir, activeName)
	closedPath := filepath.Join(dir, closedName)
	for _, path := range []string{activePath, closedPath} {
		if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := &Manager{
		recordingsDir: root,
		recorders:     map[string]*recording{cameraID: {cameraID: cameraID, path: activePath}},
		durationCache: map[string]durationCacheEntry{closedPath: {SizeBytes: 5, ModifiedUnixNS: 1, DurationSecond: 2}},
	}
	if err := m.DeleteRecording(cameraID, activeName); !errors.Is(err, ErrRecordingActive) {
		t.Fatalf("active delete err = %v, want ErrRecordingActive", err)
	}
	if err := m.DeleteRecording(cameraID, closedName); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(closedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closed recording still exists, err=%v", err)
	}
	m.durationMu.Lock()
	_, cached := m.durationCache[closedPath]
	m.durationMu.Unlock()
	if cached {
		t.Fatal("duration cache was not invalidated")
	}
}

func TestRecordingStorageStatsCountsAllMKVs(t *testing.T) {
	root := t.TempDir()
	m := &Manager{recordingsDir: root}
	for _, item := range []struct {
		path string
		size int
	}{
		{path: filepath.Join(root, "camera-1", "a.mkv"), size: 5},
		{path: filepath.Join(root, "camera-deleted", "b.MKV"), size: 7},
		{path: filepath.Join(root, "camera-1", "ignore.mp4"), size: 11},
	} {
		if err := os.MkdirAll(filepath.Dir(item.path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(item.path, make([]byte, item.size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := m.RecordingStorageStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.FileCount != 2 || stats.TotalBytes != 12 {
		t.Fatalf("stats = %#v, want 2 files / 12 bytes", stats)
	}
}
