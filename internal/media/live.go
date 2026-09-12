package media

import (
	"errors"
	"io"
	"net/http"
	"os/exec"
)

var ErrTooManyViewers = errors.New("too many live viewers for camera")

func (m *Manager) Live(w http.ResponseWriter, r *http.Request, cameraID, streamURI string) error {
	if !m.acquireViewer(cameraID) {
		return ErrTooManyViewers
	}
	defer m.releaseViewer(cameraID)

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-i", streamURI,
		"-an",
		"-vf", "fps=10,scale=960:-2:force_original_aspect_ratio=decrease",
		"-q:v", "5",
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
