package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxPlaybackViewers = 4

var (
	ErrRecordingNotFound = errors.New("recording not found")
	ErrTooManyPlayback   = errors.New("too many recording viewers")
)

type RecordingFile struct {
	CameraID       string    `json:"camera_id"`
	Name           string    `json:"name"`
	StartedAt      time.Time `json:"started_at"`
	DurationSecond float64   `json:"duration_seconds"`
	SizeBytes      int64     `json:"size_bytes"`
	Active         bool      `json:"active"`
}

func (m *Manager) ListRecordings(cameraID string) ([]RecordingFile, error) {
	dir := filepath.Join(m.recordingsDir, safePathPart(cameraID))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []RecordingFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read recordings directory: %w", err)
	}

	activePath := ""
	m.mu.Lock()
	if rec := m.recorders[cameraID]; rec != nil {
		activePath = rec.path
	}
	m.mu.Unlock()

	files := make([]RecordingFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".mkv" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, statErr := entry.Info()
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		startedAt, ok := parseRecordingStart(entry.Name())
		if !ok {
			startedAt = info.ModTime().UTC()
		}
		duration := info.ModTime().Sub(startedAt).Seconds()
		active := samePath(path, activePath)
		if active {
			duration = time.Since(startedAt).Seconds()
		}
		if duration < 0 {
			duration = 0
		}
		files = append(files, RecordingFile{
			CameraID: cameraID, Name: entry.Name(), StartedAt: startedAt,
			DurationSecond: duration, SizeBytes: info.Size(), Active: active,
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].StartedAt.After(files[j].StartedAt) })
	return files, nil
}

func parseRecordingStart(name string) (time.Time, bool) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	parsed, err := time.Parse("20060102-150405.000", base)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	cleanA, errA := filepath.Abs(a)
	cleanB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && cleanA == cleanB
}

func (m *Manager) RecordingPath(cameraID, name string) (string, error) {
	if cameraID == "" || name == "" || filepath.Base(name) != name || strings.ToLower(filepath.Ext(name)) != ".mkv" {
		return "", ErrRecordingNotFound
	}
	path := filepath.Join(m.recordingsDir, safePathPart(cameraID), name)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrRecordingNotFound
	}
	if err != nil {
		return "", fmt.Errorf("stat recording: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", ErrRecordingNotFound
	}
	return path, nil
}

func (m *Manager) RecordingSnapshot(ctx context.Context, cameraID, name string) ([]byte, error) {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-ss", "0.5",
		"-i", path,
		"-an", "-frames:v", "1",
		"-vf", "scale=640:-2:force_original_aspect_ratio=decrease",
		"-q:v", "4",
		"-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1",
	)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("recording snapshot timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("capture recording snapshot: %w", err)
	}
	if output.Len() == 0 {
		return nil, errors.New("recording snapshot is empty")
	}
	return output.Bytes(), nil
}

func (m *Manager) Playback(w http.ResponseWriter, r *http.Request, cameraID, name string, startSeconds float64) error {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return err
	}
	if startSeconds < 0 {
		startSeconds = 0
	}
	if !m.acquirePlayback() {
		return ErrTooManyPlayback
	}
	defer m.releasePlayback()

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatFloat(startSeconds, 'f', 3, 64),
		"-re",
		"-i", path,
		"-an",
		"-vf", "fps=12,scale=1280:-2:force_original_aspect_ratio=decrease",
		"-q:v", "4",
		"-f", "mpjpeg",
		"-boundary_tag", "ffmpeg",
		"pipe:1",
	}
	cmd := exec.CommandContext(r.Context(), m.ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=ffmpeg")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, stdout)
	waitErr := cmd.Wait()
	if r.Context().Err() != nil {
		return nil
	}
	if copyErr != nil {
		return copyErr
	}
	return waitErr
}

func (m *Manager) acquirePlayback() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.playbackViewers >= maxPlaybackViewers {
		return false
	}
	m.playbackViewers++
	return true
}

func (m *Manager) releasePlayback() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.playbackViewers > 0 {
		m.playbackViewers--
	}
}
