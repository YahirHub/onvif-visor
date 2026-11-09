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
	"sync"
	"time"
)

var ErrTooManyViewers = errors.New("too many live viewers for camera")

const maxJPEGFrameBytes = 8 << 20

// Un hub por cámara y perfil de calidad: un solo FFmpeg alimenta a todos los
// espectadores del mismo stream y republica cada fotograma al momento. Antes
// cada espectador lanzaba su propio FFmpeg, así que una cámara podía
// decodificarse cuatro veces a la vez entre la vista individual, la multivista
// y otros navegadores. Mantener el mismo stream para todos también evita
// cambios de relación de aspecto al agregar una cámara a la multivista.
type liveHub struct {
	key string

	mu     sync.Mutex
	subs   map[chan []byte]struct{}
	timer  *time.Timer
	ctx    context.Context
	cancel context.CancelFunc
}

func liveProfileArgs(profile string) (filter, quality string) {
	// El perfil ligero reduce FPS y resolución después de decodificar, sin
	// cambiar al substream del firmware.
	if profile == "low" {
		return "fps=5,scale=640:-2:force_original_aspect_ratio=decrease", "6"
	}
	return "fps=8,scale=960:-2:force_original_aspect_ratio=decrease", "5"
}

func (m *Manager) hubFor(cameraID, profile, streamURI string) *liveHub {
	key := cameraID + "|" + profile
	m.hubsMu.Lock()
	defer m.hubsMu.Unlock()
	if h, ok := m.hubs[key]; ok && h.ctx.Err() == nil {
		return h
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &liveHub{key: key, subs: make(map[chan []byte]struct{}), ctx: ctx, cancel: cancel}
	m.hubs[key] = h
	go m.runHub(h, streamURI, profile)
	return h
}

// runHub mantiene vivo el FFmpeg del hub mientras tenga sentido: si la cámara
// se corta, el proceso se relanza con espera creciente y los espectadores no
// pierden su conexión, solo dejan de recibir fotogramas unos segundos. El
// navegador conserva el último fotograma del MJPEG, sin error ni corte seco.
func (m *Manager) runHub(h *liveHub, streamURI, profile string) {
	backoff := time.Second
	for {
		streamed := m.streamHubOnce(h, streamURI, profile)
		if h.ctx.Err() != nil {
			return
		}
		if streamed {
			backoff = time.Second
		}
		select {
		case <-h.ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

// liveFFmpegArgs son los argumentos del FFmpeg del directo. No usar opciones
// nuevas sin probarlas en el FFmpeg del host: -rw_timeout no existe en
// FFmpeg 9 y el proceso moría al arrancar, dejando el directo sin fotogramas.
func liveFFmpegArgs(streamURI, profile string) []string {
	filter, quality := liveProfileArgs(profile)
	return []string{
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
}

func (m *Manager) streamHubOnce(h *liveHub, streamURI, profile string) bool {
	cmd := exec.CommandContext(h.ctx, m.ffmpeg, liveFFmpegArgs(streamURI, profile)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return false
	}
	frames := make(chan []byte, 1)
	readDone := make(chan error, 1)
	go readLatestJPEGFrames(stdout, frames, readDone)
	streamed := false
	// Vigilante de silencio: si la conexión RTSP se cuelga sin datos y sin
	// morir, a los 15 s se mata el proceso y runHub lo relanza.
	idle := time.NewTimer(15 * time.Second)
	defer idle.Stop()
	for {
		select {
		case <-h.ctx.Done():
			_ = cmd.Wait()
			return streamed
		case <-readDone:
			_ = cmd.Wait()
			return streamed
		case <-idle.C:
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return streamed
		case frame := <-frames:
			if len(frame) == 0 {
				continue
			}
			streamed = true
			h.publish(frame)
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(15 * time.Second)
		}
	}
}

func (h *liveHub) publish(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		// Cada suscriptor conserva solo el fotograma más reciente: un cliente
		// lento salta fotogramas viejos en vez de acumular retraso.
		pushLatestFrame(ch, frame)
	}
}

func (h *liveHub) subscribe() chan []byte {
	ch := make(chan []byte, 1)
	h.mu.Lock()
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (m *Manager) unsubscribe(h *liveHub, ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	empty := len(h.subs) == 0
	if empty && h.timer == nil {
		// Gracia de 3 s antes de matar el FFmpeg: navegar entre la vista de
		// cámara y la multivista no debe relanzar el proceso cada vez.
		h.timer = time.AfterFunc(3*time.Second, func() { m.stopHub(h) })
	}
	h.mu.Unlock()
}

func (m *Manager) stopHub(h *liveHub) {
	h.mu.Lock()
	empty := len(h.subs) == 0
	h.mu.Unlock()
	if !empty {
		return
	}
	m.hubsMu.Lock()
	if m.hubs[h.key] == h {
		delete(m.hubs, h.key)
	}
	m.hubsMu.Unlock()
	h.cancel()
}

func (m *Manager) Live(w http.ResponseWriter, r *http.Request, cameraID, streamURI string) error {
	if !m.acquireViewer(cameraID) {
		return ErrTooManyViewers
	}
	defer m.releaseViewer(cameraID)

	profile := "full"
	if strings.EqualFold(r.URL.Query().Get("quality"), "low") {
		profile = "low"
	}
	hub := m.hubFor(cameraID, profile, streamURI)
	frames := hub.subscribe()
	defer m.unsubscribe(hub, frames)

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=onvifframe")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	for {
		select {
		case <-r.Context().Done():
			return nil
		case frame := <-frames:
			if len(frame) == 0 {
				continue
			}
			if _, err := fmt.Fprintf(w, "--onvifframe\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame)); err != nil {
				return err
			}
			if _, err := w.Write(frame); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\r\n"); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
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
