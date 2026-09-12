package media

import (
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
