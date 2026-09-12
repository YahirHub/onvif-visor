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

const (
	telegramClipMinDuration = 3 * time.Second
	telegramClipMaxDuration = 60 * time.Second
	telegramClipWarmup      = 500 * time.Millisecond
)

func normalizeTelegramClipDuration(duration time.Duration) time.Duration {
	if duration < telegramClipMinDuration {
		return telegramClipMinDuration
	}
	if duration > telegramClipMaxDuration {
		return telegramClipMaxDuration
	}
	return duration
}

func (m *Manager) CaptureClip(ctx context.Context, streamURI string, duration time.Duration) (string, error) {
	duration = normalizeTelegramClipDuration(duration)
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
	clipCtx, cancel := context.WithTimeout(ctx, duration+20*time.Second)
	defer cancel()

	// Para clips compartibles se prioriza estabilidad sobre la latencia mínima del live:
	// timestamps de reloj + setpts evitan MP4 de ~1 s cuando una cámara entrega PTS
	// discontinuos, y un warm-up corto descarta el primer medio segundo de decodificación.
	filter := fmt.Sprintf(
		"trim=start=%.3f,setpts=PTS-STARTPTS,fps=12,scale=min(1280\\,iw):-2,format=yuv420p",
		telegramClipWarmup.Seconds(),
	)
	cmd := exec.CommandContext(clipCtx, m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-thread_queue_size", "256",
		"-use_wallclock_as_timestamps", "1",
		"-fflags", "+genpts+discardcorrupt",
		"-analyzeduration", "1000000",
		"-probesize", "1000000",
		"-i", streamURI,
		"-map", "0:v:0",
		"-t", fmt.Sprintf("%.3f", duration.Seconds()),
		"-an",
		"-vf", filter,
		"-fps_mode", "cfr",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-maxrate", "1800k",
		"-bufsize", "3600k",
		"-g", "24",
		"-keyint_min", "24",
		"-sc_threshold", "0",
		"-pix_fmt", "yuv420p",
		"-threads", "2",
		"-avoid_negative_ts", "make_zero",
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
	if info.Size() < 1024 {
		_ = os.Remove(path)
		return "", errors.New("captured clip is empty or incomplete")
	}
	return path, nil
}
