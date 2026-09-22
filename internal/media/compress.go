package media

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	minCompressionBytes      = 2 << 20
	compressionRatioKeep     = 0.97
	maxCompressionClipLength = 6 * time.Hour
)

func (m *Manager) queueCompression(path string) {
	if path == "" {
		return
	}
	go func() {
		m.compressionSem <- struct{}{}
		defer func() { <-m.compressionSem }()
		_ = m.compressRecording(path)
	}()
}

func (m *Manager) compressRecording(path string) error {
	original, err := os.Stat(path)
	if err != nil {
		return err
	}
	if original.Size() < minCompressionBytes || !compressionSuitableForClip(path, original.ModTime()) {
		return nil
	}

	tmp := path + ".compressing.tmp"
	_ = os.Remove(tmp)
	cmd := exec.Command(m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-i", path,
		"-map", "0:v:0", "-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "24",
		"-pix_fmt", "yuv420p",
		"-threads", "1",
		"-c:a", "copy",
		"-flush_packets", "1",
		"-cluster_time_limit", "1000",
		"-cluster_size_limit", "1048576",
		"-f", "matroska",
		"-y", tmp,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("compress recording: %w", err)
	}

	compressed, err := os.Stat(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if !shouldReplaceWithCompressed(original.Size(), compressed.Size()) {
		_ = os.Remove(tmp)
		return nil
	}

	file, err := os.OpenFile(tmp, os.O_RDWR, 0)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chtimes(tmp, original.ModTime(), original.ModTime()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncParentDirectory(path)
}

func compressionSuitableForClip(path string, modified time.Time) bool {
	started, ok := parseRecordingStart(filepath.Base(path))
	if !ok {
		return true
	}
	duration := modified.Sub(started)
	return duration <= 0 || duration <= maxCompressionClipLength
}

func shouldReplaceWithCompressed(originalBytes, compressedBytes int64) bool {
	if originalBytes <= 0 || compressedBytes <= 0 {
		return false
	}
	return float64(compressedBytes) < float64(originalBytes)*compressionRatioKeep
}

func syncParentDirectory(path string) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
