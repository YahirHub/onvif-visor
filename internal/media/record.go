package media

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *Manager) Trigger(cameraID, streamURI string, postRoll time.Duration) (RecordingStatus, error) {
	if postRoll <= 0 {
		postRoll = 20 * time.Second
	}
	m.mu.Lock()
	if rec, ok := m.recorders[cameraID]; ok {
		rec.stopAfter = time.Now().Add(postRoll)
		status := RecordingStatus{
			CameraID: cameraID, Active: true, Path: rec.path,
			StartedAt: rec.startedAt, StopAfter: rec.stopAfter,
		}
		m.mu.Unlock()
		return status, nil
	}

	dir := filepath.Join(m.recordingsDir, safePathPart(cameraID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.mu.Unlock()
		return RecordingStatus{}, fmt.Errorf("create recording directory: %w", err)
	}
	startedAt := time.Now().UTC()
	path := filepath.Join(dir, startedAt.Format("20060102-150405.000")+".mkv")
	cmd := exec.Command(m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", streamURI,
		"-map", "0:v:0", "-map", "0:a?",
		"-c", "copy",
		"-y", path,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		m.mu.Unlock()
		return RecordingStatus{}, fmt.Errorf("start ffmpeg recording: %w", err)
	}
	rec := &recording{
		cameraID:  cameraID,
		path:      path,
		startedAt: startedAt,
		stopAfter: time.Now().Add(postRoll),
		cmd:       cmd,
		done:      make(chan error, 1),
	}
	m.recorders[cameraID] = rec
	m.mu.Unlock()

	go m.monitorRecording(rec)
	return RecordingStatus{
		CameraID: cameraID, Active: true, Path: path,
		StartedAt: startedAt, StopAfter: rec.stopAfter,
	}, nil
}

func (m *Manager) monitorRecording(rec *recording) {
	go func() { rec.done <- rec.cmd.Wait() }()

	for {
		m.mu.Lock()
		current, ok := m.recorders[rec.cameraID]
		if !ok || current != rec {
			m.mu.Unlock()
			return
		}
		remaining := time.Until(rec.stopAfter)
		m.mu.Unlock()
		if remaining < 100*time.Millisecond {
			remaining = 100 * time.Millisecond
		}

		timer := time.NewTimer(remaining)
		select {
		case <-rec.done:
			timer.Stop()
			m.removeRecorder(rec)
			return
		case <-timer.C:
			m.mu.Lock()
			shouldStop := time.Now().After(rec.stopAfter) || time.Now().Equal(rec.stopAfter)
			m.mu.Unlock()
			if !shouldStop {
				continue
			}
			m.stopProcess(rec)
			m.removeRecorder(rec)
			return
		}
	}
}

func (m *Manager) stopProcess(rec *recording) {
	if rec.cmd.Process == nil {
		return
	}
	_ = rec.cmd.Process.Signal(os.Interrupt)
	select {
	case <-rec.done:
		return
	case <-time.After(3 * time.Second):
		_ = rec.cmd.Process.Kill()
		select {
		case <-rec.done:
		case <-time.After(time.Second):
		}
	}
}

func (m *Manager) removeRecorder(rec *recording) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.recorders[rec.cameraID]; ok && current == rec {
		delete(m.recorders, rec.cameraID)
	}
}

func (m *Manager) Stop(cameraID string) {
	m.mu.Lock()
	rec := m.recorders[cameraID]
	m.mu.Unlock()
	if rec != nil {
		m.stopProcess(rec)
		m.removeRecorder(rec)
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	recs := make([]*recording, 0, len(m.recorders))
	for _, rec := range m.recorders {
		recs = append(recs, rec)
	}
	m.mu.Unlock()
	for _, rec := range recs {
		m.stopProcess(rec)
		m.removeRecorder(rec)
	}
}

func safePathPart(value string) string {
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "camera"
	}
	return string(out)
}
