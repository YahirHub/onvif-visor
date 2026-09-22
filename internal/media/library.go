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
	"syscall"
	"time"
)

const maxPlaybackViewers = 4

var (
	ErrRecordingNotFound = errors.New("recording not found")
	ErrRecordingActive   = errors.New("recording is active")
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

type RecordingStorageStats struct {
	TotalBytes         int64   `json:"total_bytes"`
	FileCount          int     `json:"file_count"`
	DiskTotalBytes     int64   `json:"disk_total_bytes"`
	DiskAvailableBytes int64   `json:"disk_available_bytes"`
	DiskUsedPercent    float64 `json:"disk_used_percent"`
}

type durationCacheEntry struct {
	SizeBytes      int64
	ModifiedUnixNS int64
	DurationSecond float64
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

func (m *Manager) RecordingStorageStats() (RecordingStorageStats, error) {
	var stats RecordingStorageStats
	err := filepath.WalkDir(m.recordingsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) && path == m.recordingsDir {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".mkv" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		stats.TotalBytes += info.Size()
		stats.FileCount++
		return nil
	})
	if err != nil {
		return RecordingStorageStats{}, fmt.Errorf("scan recordings storage: %w", err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(m.recordingsDir, &fs); err == nil {
		blockSize := int64(fs.Bsize)
		stats.DiskTotalBytes = int64(fs.Blocks) * blockSize
		stats.DiskAvailableBytes = int64(fs.Bavail) * blockSize
		if stats.DiskTotalBytes > 0 {
			used := stats.DiskTotalBytes - stats.DiskAvailableBytes
			stats.DiskUsedPercent = float64(used) / float64(stats.DiskTotalBytes) * 100
		}
	}
	return stats, nil
}

func (m *Manager) RecordingMetadata(ctx context.Context, cameraID, name string) (RecordingFile, error) {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return RecordingFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RecordingFile{}, ErrRecordingNotFound
		}
		return RecordingFile{}, fmt.Errorf("stat recording metadata: %w", err)
	}
	startedAt, ok := parseRecordingStart(name)
	if !ok {
		startedAt = info.ModTime().UTC()
	}
	active := m.recordingIsActive(cameraID, path)
	duration := info.ModTime().Sub(startedAt).Seconds()
	if active {
		duration = time.Since(startedAt).Seconds()
	} else if exact, exactErr := m.exactRecordingDuration(ctx, path, info); exactErr == nil {
		duration = exact
	}
	if duration < 0 {
		duration = 0
	}
	return RecordingFile{
		CameraID: cameraID, Name: name, StartedAt: startedAt,
		DurationSecond: duration, SizeBytes: info.Size(), Active: active,
	}, nil
}

func (m *Manager) exactRecordingDuration(ctx context.Context, path string, info os.FileInfo) (float64, error) {
	if m.ffprobe == "" {
		return 0, errors.New("ffprobe not available")
	}
	modified := info.ModTime().UnixNano()
	m.durationMu.Lock()
	if cached, ok := m.durationCache[path]; ok && cached.SizeBytes == info.Size() && cached.ModifiedUnixNS == modified {
		m.durationMu.Unlock()
		return cached.DurationSecond, nil
	}
	m.durationMu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, m.ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		if probeCtx.Err() != nil {
			return 0, fmt.Errorf("probe recording duration: %w", probeCtx.Err())
		}
		return 0, fmt.Errorf("probe recording duration: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || duration < 0 {
		return 0, errors.New("invalid recording duration")
	}
	m.durationMu.Lock()
	if m.durationCache == nil {
		m.durationCache = make(map[string]durationCacheEntry)
	}
	m.durationCache[path] = durationCacheEntry{SizeBytes: info.Size(), ModifiedUnixNS: modified, DurationSecond: duration}
	m.durationMu.Unlock()
	return duration, nil
}

func (m *Manager) recordingIsActive(cameraID, path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.recorders[cameraID]
	return rec != nil && samePath(rec.path, path)
}

func (m *Manager) invalidateRecordingDuration(path string) {
	m.durationMu.Lock()
	delete(m.durationCache, path)
	m.durationMu.Unlock()
}

func (m *Manager) RecordingExists(cameraID, name string) bool {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (m *Manager) DeleteRecording(cameraID, name string) error {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return err
	}
	if m.recordingIsActive(cameraID, path) {
		return ErrRecordingActive
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrRecordingNotFound
		}
		return fmt.Errorf("delete recording: %w", err)
	}
	m.invalidateRecordingDuration(path)
	_ = syncParentDirectory(path)
	return nil
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

func (m *Manager) RecordingSnapshot(ctx context.Context, cameraID, name string, offsetSeconds float64) ([]byte, error) {
	path, err := m.RecordingPath(cameraID, name)
	if err != nil {
		return nil, err
	}
	if offsetSeconds < 0 {
		offsetSeconds = 0
	}
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatFloat(offsetSeconds, 'f', 3, 64),
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
