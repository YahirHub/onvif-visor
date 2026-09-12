package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

var ErrTooManyViewers = errors.New("too many live viewers for camera")

const maxJPEGFrameBytes = 8 << 20

func (m *Manager) Live(w http.ResponseWriter, r *http.Request, cameraID, streamURI string) error {
	if !m.acquireViewer(cameraID) {
		return ErrTooManyViewers
	}
	defer m.releaseViewer(cameraID)

	lowBandwidth := strings.EqualFold(r.URL.Query().Get("quality"), "low")
	if lowBandwidth {
		streamURI = motionStreamURI(streamURI)
	}
	filter := "fps=8,scale=960:-2:force_original_aspect_ratio=decrease"
	quality := "5"
	if lowBandwidth {
		filter = "fps=5,scale=640:-2:force_original_aspect_ratio=decrease"
		quality = "6"
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-analyzeduration", "0",
		"-probesize", "32768",
		"-max_delay", "0",
		"-i", streamURI,
		"-an",
		"-vf", filter,
		"-q:v", quality,
		"-threads", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	}
	streamCtx, cancelStream := context.WithCancel(r.Context())
	defer cancelStream()
	cmd := exec.CommandContext(streamCtx, m.ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}

	frames := make(chan []byte, 1)
	readDone := make(chan error, 1)
	go readLatestJPEGFrames(stdout, frames, readDone)

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=onvifframe")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	var streamErr error
streamLoop:
	for {
		select {
		case <-r.Context().Done():
			break streamLoop
		case err := <-readDone:
			streamErr = err
			break streamLoop
		case frame := <-frames:
			if len(frame) == 0 {
				continue
			}
			if _, err := fmt.Fprintf(w, "--onvifframe\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame)); err != nil {
				streamErr = err
				break streamLoop
			}
			if _, err := w.Write(frame); err != nil {
				streamErr = err
				break streamLoop
			}
			if _, err := io.WriteString(w, "\r\n"); err != nil {
				streamErr = err
				break streamLoop
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}

	cancelStream()
	waitErr := cmd.Wait()
	if r.Context().Err() != nil {
		return nil
	}
	if streamErr != nil {
		return streamErr
	}
	return waitErr
}

func readLatestJPEGFrames(src io.Reader, frames chan []byte, done chan<- error) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 128<<10), maxJPEGFrameBytes)
	scanner.Split(splitJPEGFrames)
	for scanner.Scan() {
		pushLatestFrame(frames, append([]byte(nil), scanner.Bytes()...))
	}
	done <- scanner.Err()
}

func pushLatestFrame(frames chan []byte, frame []byte) {
	select {
	case frames <- frame:
		return
	default:
	}
	select {
	case <-frames:
	default:
	}
	select {
	case frames <- frame:
	default:
	}
}

func splitJPEGFrames(data []byte, atEOF bool) (advance int, token []byte, err error) {
	start := bytes.Index(data, []byte{0xff, 0xd8})
	if start < 0 {
		if atEOF {
			return len(data), nil, nil
		}
		if len(data) > 1 {
			return len(data) - 1, nil, nil
		}
		return 0, nil, nil
	}
	if start > 0 {
		return start, nil, nil
	}
	end := bytes.Index(data[2:], []byte{0xff, 0xd9})
	if end >= 0 {
		end += 4
		return end, data[:end], nil
	}
	if atEOF {
		return len(data), nil, io.ErrUnexpectedEOF
	}
	return 0, nil, nil
}

func (m *Manager) acquireViewer(cameraID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.liveViewers[cameraID] >= maxLiveViewersPerCamera {
		return false
	}
	m.liveViewers[cameraID]++
	return true
}

func (m *Manager) releaseViewer(cameraID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.liveViewers[cameraID] <= 1 {
		delete(m.liveViewers, cameraID)
		return
	}
	m.liveViewers[cameraID]--
}
