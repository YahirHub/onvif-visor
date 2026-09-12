package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const snapshotTimeout = 8 * time.Second

func (m *Manager) Snapshot(ctx context.Context, streamURI string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", streamURI,
		"-an",
		"-frames:v", "1",
		"-vf", "scale=640:-2:force_original_aspect_ratio=decrease",
		"-q:v", "4",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, m.ffmpeg, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("snapshot timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("capture snapshot: %w", err)
	}
	if output.Len() == 0 {
		return nil, fmt.Errorf("capture snapshot: empty image")
	}
	return output.Bytes(), nil
}
