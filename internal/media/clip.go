package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

func (m *Manager) CaptureClip(ctx context.Context, streamURI string, duration time.Duration) (string, error) {
	if duration < 3*time.Second {
		duration = 3 * time.Second
	}
	if duration > 10*time.Second {
		duration = 10 * time.Second
	}
	file, err := os.CreateTemp("", "onvif-telegram-*.mp4")
	if err != nil {
		return "", fmt.Errorf("create clip temp file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	// Mantener el archivo creado con permisos privados; FFmpeg lo trunca con -y.
	clipCtx, cancel := context.WithTimeout(ctx, duration+12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(clipCtx, m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-i", streamURI,
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
		"-an",
		"-vf", "fps=10,scale=960:-2:force_original_aspect_ratio=decrease",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "27",
		"-pix_fmt", "yuv420p",
		"-threads", "1",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y", path,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		_ = os.Remove(path)
		if clipCtx.Err() != nil {
			return "", fmt.Errorf("capture clip timeout: %w", clipCtx.Err())
		}
		return "", fmt.Errorf("capture clip: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if info.Size() == 0 {
		_ = os.Remove(path)
		return "", errors.New("captured clip is empty")
	}
	return path, nil
}
