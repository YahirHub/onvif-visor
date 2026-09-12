package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
)

const (
	motionWidth          = 160
	motionHeight         = 90
	motionFrameSize      = motionWidth * motionHeight
	motionPixelThreshold = 22
	motionStartRatio     = 0.035
	motionStartFrames    = 2
	motionStopFrames     = 4
)

// WatchMotion observes a low-bandwidth grayscale stream and reports motion state
// transitions. It is used as a fallback when a camera advertises ONVIF events
// but its PullPoint implementation is broken or unavailable.
func (m *Manager) WatchMotion(ctx context.Context, streamURI string, callback func(active bool, score float64)) error {
	candidate := motionStreamURI(streamURI)
	if err := m.watchMotionStream(ctx, candidate, callback); err == nil || ctx.Err() != nil || candidate == streamURI {
		return err
	}
	return m.watchMotionStream(ctx, streamURI, callback)
}

func (m *Manager) watchMotionStream(ctx context.Context, streamURI string, callback func(active bool, score float64)) error {
	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", streamURI,
		"-an",
		"-vf", "fps=2,scale=160:90:force_original_aspect_ratio=decrease,pad=160:90:(ow-iw)/2:(oh-ih)/2,format=gray",
		"-pix_fmt", "gray",
		"-f", "rawvideo", "pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("motion stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start motion detector: %w", err)
	}

	previous := make([]byte, motionFrameSize)
	current := make([]byte, motionFrameSize)
	if _, err := io.ReadFull(stdout, previous); err != nil {
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("read first motion frame: %w: %s", err, stderr.String())
	}

	active := false
	activeFrames := 0
	quietFrames := 0
	for {
		if _, err := io.ReadFull(stdout, current); err != nil {
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("motion stream ended: %v: %s", waitErr, stderr.String())
			}
			return fmt.Errorf("read motion frame: %w", err)
		}

		score := frameDifferenceScore(previous, current)
		copy(previous, current)

		if score >= motionStartRatio {
			activeFrames++
			quietFrames = 0
			if !active && activeFrames >= motionStartFrames {
				active = true
				if callback != nil {
					callback(true, score)
				}
			}
			continue
		}

		activeFrames = 0
		if active {
			quietFrames++
			if quietFrames >= motionStopFrames {
				active = false
				quietFrames = 0
				if callback != nil {
					callback(false, score)
				}
			}
		}
	}
}

func frameDifferenceScore(previous, current []byte) float64 {
	if len(previous) == 0 || len(previous) != len(current) {
		return 0
	}
	var shiftSum int64
	for i := range previous {
		shiftSum += int64(int(current[i]) - int(previous[i]))
	}
	globalShift := float64(shiftSum) / float64(len(previous))
	changed := 0
	for i := range previous {
		delta := float64(int(current[i])-int(previous[i])) - globalShift
		if delta < 0 {
			delta = -delta
		}
		if delta >= motionPixelThreshold {
			changed++
		}
	}
	return float64(changed) / float64(len(previous))
}

func motionStreamURI(streamURI string) string {
	parsed, err := url.Parse(streamURI)
	if err != nil {
		return streamURI
	}
	query := parsed.Query()
	if query.Get("subtype") == "0" {
		query.Set("subtype", "1")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return streamURI
}
