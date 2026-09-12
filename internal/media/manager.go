package media

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const maxLiveViewersPerCamera = 4

type Manager struct {
	ffmpeg        string
	recordingsDir string

	mu              sync.Mutex
	liveViewers     map[string]int
	playbackViewers int
	recorders       map[string]*recording
}

type RecordingStatus struct {
	CameraID  string    `json:"camera_id"`
	Active    bool      `json:"active"`
	Path      string    `json:"path,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
	StopAfter time.Time `json:"stop_after,omitempty"`
}

type recording struct {
	cameraID  string
	path      string
	startedAt time.Time
	stopAfter time.Time
	cmd       *exec.Cmd
	done      chan error
}

func New(ffmpegPath, recordingsDir string) (*Manager, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	resolved, err := exec.LookPath(ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", err)
	}
	if recordingsDir == "" {
		return nil, errors.New("recordings directory is required")
	}
	return &Manager{
		ffmpeg:        resolved,
		recordingsDir: filepath.Clean(recordingsDir),
		liveViewers:   make(map[string]int),
		recorders:     make(map[string]*recording),
	}, nil
}

func (m *Manager) Status(cameraID string) RecordingStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.recorders[cameraID]
	if !ok {
		return RecordingStatus{CameraID: cameraID}
	}
	return RecordingStatus{
		CameraID:  cameraID,
		Active:    true,
		Path:      rec.path,
		StartedAt: rec.startedAt,
		StopAfter: rec.stopAfter,
	}
}

func (m *Manager) AllStatus() []RecordingStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RecordingStatus, 0, len(m.recorders))
	for id, rec := range m.recorders {
		out = append(out, RecordingStatus{
			CameraID: id, Active: true, Path: rec.path,
			StartedAt: rec.startedAt, StopAfter: rec.stopAfter,
		})
	}
	return out
}
