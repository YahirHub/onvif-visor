package analytics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"onvif-visor/internal/media"
)

const (
	frameSize          = InputWidth * InputHeight * 3
	scanInterval       = 15 * time.Second
	liveRestartDelay   = 3 * time.Second
	progressEveryFrame = 10
)

type Config struct {
	DataDir     string
	FFmpeg      string
	ModelPath   string
	RuntimePath string
	Media       *media.Manager
}

type CameraSource struct {
	ID        string
	StreamURI string
}

type PersonFrame struct {
	JPEG       []byte
	DetectedAt time.Time
	Confidence float64
}

type Status struct {
	Enabled          bool    `json:"enabled"`
	Ready            bool    `json:"ready"`
	Model            string  `json:"model"`
	SampleFPS        int     `json:"sample_fps"`
	QueueDepth       int     `json:"queue_depth"`
	Analyzing        bool    `json:"analyzing"`
	CameraID         string  `json:"camera_id,omitempty"`
	RecordingName    string  `json:"recording_name,omitempty"`
	ProgressSecond   float64 `json:"progress_seconds,omitempty"`
	DurationSecond   float64 `json:"duration_seconds,omitempty"`
	LastError        string  `json:"last_error,omitempty"`
	RuntimeAvailable bool    `json:"runtime_available"`
}

type analysisJob struct {
	CameraID      string
	RecordingName string
	Path          string
	SizeBytes     int64
	ModifiedNS    int64
	Duration      float64
}

type Manager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	ffmpeg   string
	ffprobe  string
	model    string
	runtime  string
	media    *media.Manager
	store    *Store
	detector Detector

	inferMu      sync.Mutex
	mu           sync.Mutex
	sources      map[string]CameraSource
	live         map[string]context.CancelFunc
	queued       map[string]bool
	personFrames map[string]PersonFrame
	status       Status
	jobs         chan analysisJob
	wg           sync.WaitGroup
}

func New(cfg Config) (*Manager, error) {
	if cfg.Media == nil {
		return nil, errors.New("analytics media manager is required")
	}
	if strings.TrimSpace(cfg.FFmpeg) == "" {
		cfg.FFmpeg = "ffmpeg"
	}
	store, err := OpenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ffprobe := ""
	if resolved, lookupErr := exec.LookPath(cfg.FFmpeg); lookupErr == nil {
		sibling := filepath.Join(filepath.Dir(resolved), "ffprobe")
		if info, statErr := os.Stat(sibling); statErr == nil && !info.IsDir() {
			ffprobe = sibling
		}
	}
	if ffprobe == "" {
		ffprobe, _ = exec.LookPath("ffprobe")
	}
	m := &Manager{
		ctx: ctx, cancel: cancel, ffmpeg: cfg.FFmpeg, ffprobe: ffprobe, model: cfg.ModelPath,
		runtime: ResolveRuntimePath(cfg.RuntimePath), media: cfg.Media, store: store,
		sources: make(map[string]CameraSource), live: make(map[string]context.CancelFunc),
		queued: make(map[string]bool), personFrames: make(map[string]PersonFrame), jobs: make(chan analysisJob, 128),
		status: Status{Enabled: true, Model: ModelName, SampleFPS: 1},
	}
	detector, detectorErr := NewDetector(m.runtime, cfg.ModelPath)
	if detectorErr != nil {
		m.status.Ready = false
		m.status.RuntimeAvailable = m.runtime != ""
		m.status.LastError = detectorErr.Error()
	} else {
		m.detector = detector
		m.status.Ready = true
		m.status.RuntimeAvailable = true
		m.wg.Add(2)
		go m.worker()
		go m.scanner()
	}
	return m, nil
}

func (m *Manager) Close() error {
	m.cancel()
	m.mu.Lock()
	for _, cancel := range m.live {
		cancel()
	}
	m.live = make(map[string]context.CancelFunc)
	m.mu.Unlock()
	m.wg.Wait()
	if m.detector != nil {
		_ = m.detector.Close()
	}
	return m.store.Close()
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.status
	status.QueueDepth = len(m.jobs)
	return status
}

func (m *Manager) Segments(ctx context.Context, cameraID string) ([]PersonSegment, error) {
	return m.store.Segments(ctx, cameraID)
}

func (m *Manager) Analyses(ctx context.Context, cameraID string) ([]RecordingAnalysis, error) {
	return m.store.Analyses(ctx, cameraID)
}

func (m *Manager) ForgetRecording(ctx context.Context, cameraID, recordingName string) error {
	return m.store.DeleteRecording(ctx, cameraID, recordingName)
}

func (m *Manager) WaitForPersonFrame(ctx context.Context, cameraID string, notBefore time.Time) (PersonFrame, bool) {
	check := func() (PersonFrame, bool) {
		m.mu.Lock()
		defer m.mu.Unlock()
		frame, ok := m.personFrames[cameraID]
		if !ok || frame.DetectedAt.Before(notBefore) || len(frame.JPEG) == 0 {
			return PersonFrame{}, false
		}
		copyFrame := frame
		copyFrame.JPEG = append([]byte(nil), frame.JPEG...)
		return copyFrame, true
	}
	if frame, ok := check(); ok {
		return frame, true
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return PersonFrame{}, false
		case <-ticker.C:
			if frame, ok := check(); ok {
				return frame, true
			}
		}
	}
}

func (m *Manager) SyncCamera(source CameraSource) {
	if !m.Status().Ready || strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.StreamURI) == "" {
		return
	}
	m.mu.Lock()
	previous, exists := m.sources[source.ID]
	m.sources[source.ID] = source
	if exists && previous.StreamURI == source.StreamURI {
		m.mu.Unlock()
		return
	}
	if cancel := m.live[source.ID]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.live[source.ID] = cancel
	m.mu.Unlock()
	m.wg.Add(1)
	go m.liveLoop(ctx, source)
}

func (m *Manager) RemoveCamera(cameraID string) {
	m.mu.Lock()
	delete(m.sources, cameraID)
	if cancel := m.live[cameraID]; cancel != nil {
		cancel()
	}
	delete(m.live, cameraID)
	delete(m.personFrames, cameraID)
	m.mu.Unlock()
}

func (m *Manager) ScanNow() {
	go m.scanOnce(false)
}

func (m *Manager) Reindex() {
	// Marcar lo ya indexado como pendiente permite reanalizar bibliotecas grandes
	// en tandas sin perder trabajos cuando el canal está momentáneamente lleno.
	go func() {
		if err := m.store.ResetAnalyses(m.ctx); err != nil {
			m.setLastError(err)
			return
		}
		m.scanOnce(false)
	}()
}

func (m *Manager) scanner() {
	defer m.wg.Done()
	m.scanOnce(false)
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.scanOnce(false)
		}
	}
}

func (m *Manager) scanOnce(force bool) {
	m.mu.Lock()
	sources := make([]CameraSource, 0, len(m.sources))
	for _, source := range m.sources {
		sources = append(sources, source)
	}
	m.mu.Unlock()
	for _, source := range sources {
		if m.ctx.Err() != nil {
			return
		}
		files, err := m.media.ListRecordings(source.ID)
		if err != nil {
			m.setLastError(err)
			continue
		}
		for i := len(files) - 1; i >= 0; i-- { // oldest first
			file := files[i]
			if file.Active {
				continue
			}
			path, err := m.media.RecordingPath(source.ID, file.Name)
			if err != nil {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if !force {
				needs, err := m.store.NeedsAnalysis(m.ctx, source.ID, file.Name, info.Size(), info.ModTime().UnixNano(), ModelName)
				if err != nil {
					m.setLastError(err)
					continue
				}
				if !needs {
					continue
				}
			}
			job := analysisJob{CameraID: source.ID, RecordingName: file.Name, Path: path, SizeBytes: info.Size(), ModifiedNS: info.ModTime().UnixNano(), Duration: file.DurationSecond}
			m.enqueue(job)
		}
	}
}

func (m *Manager) enqueue(job analysisJob) {
	key := job.CameraID + "\x00" + job.RecordingName
	m.mu.Lock()
	if m.queued[key] {
		m.mu.Unlock()
		return
	}
	m.queued[key] = true
	m.mu.Unlock()
	select {
	case m.jobs <- job:
	case <-m.ctx.Done():
		m.mu.Lock()
		delete(m.queued, key)
		m.mu.Unlock()
	default:
		m.mu.Lock()
		delete(m.queued, key)
		m.mu.Unlock()
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case job := <-m.jobs:
			m.analyze(job)
			key := job.CameraID + "\x00" + job.RecordingName
			m.mu.Lock()
			delete(m.queued, key)
			m.mu.Unlock()
		}
	}
}

func (m *Manager) analyze(job analysisJob) {
	if err := m.store.MarkAnalyzing(m.ctx, job.CameraID, job.RecordingName, job.SizeBytes, job.ModifiedNS, job.Duration, ModelName); err != nil {
		m.setLastError(err)
		return
	}
	m.setAnalyzing(job, 0)
	points := make([]DetectionPoint, 0)
	filter := fmt.Sprintf("fps=1,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=bgr24", InputWidth, InputHeight, InputWidth, InputHeight)
	cmd := exec.CommandContext(m.ctx, m.ffmpeg, "-hide_banner", "-loglevel", "error", "-threads", "1", "-i", job.Path, "-an", "-vf", filter, "-pix_fmt", "bgr24", "-f", "rawvideo", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.failJob(job, err)
		return
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		m.failJob(job, err)
		return
	}
	frame := make([]byte, frameSize)
	frameIndex := 0
	for {
		_, readErr := io.ReadFull(stdout, frame)
		if readErr != nil {
			waitErr := cmd.Wait()
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				if waitErr != nil && m.ctx.Err() == nil {
					m.failJob(job, fmt.Errorf("ffmpeg analytics: %v: %s", waitErr, strings.TrimSpace(stderr.String())))
					return
				}
				break
			}
			m.failJob(job, readErr)
			return
		}
		confidence, found, detectErr := m.detect(frame)
		if detectErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			m.failJob(job, detectErr)
			return
		}
		second := float64(frameIndex)
		if found {
			points = append(points, DetectionPoint{Second: second, Confidence: float64(confidence)})
		}
		frameIndex++
		if frameIndex%progressEveryFrame == 0 {
			_ = m.store.UpdateProgress(m.ctx, job.CameraID, job.RecordingName, second)
			m.setAnalyzing(job, second)
		}
		if m.ctx.Err() != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return
		}
		// Cede CPU a live/recording en este servidor pequeño.
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(job.Path); errors.Is(err, os.ErrNotExist) {
		_ = m.store.DeleteRecording(context.Background(), job.CameraID, job.RecordingName)
		m.clearAnalyzing()
		return
	}
	segments := MergeDetectionPoints(points, 3)
	analysis := RecordingAnalysis{CameraID: job.CameraID, RecordingName: job.RecordingName, SizeBytes: job.SizeBytes, ModifiedUnixNS: job.ModifiedNS, DurationSecond: maxFloat(job.Duration, float64(frameIndex)), Status: "complete", ProgressSecond: float64(frameIndex), Model: ModelName, AnalyzedAt: time.Now().UTC()}
	if err := m.store.Complete(m.ctx, analysis, segments); err != nil {
		m.setLastError(err)
	}
	m.clearAnalyzing()
}

func (m *Manager) detect(frame []byte) (float32, bool, error) {
	m.inferMu.Lock()
	defer m.inferMu.Unlock()
	if m.detector == nil {
		return 0, false, ErrInferenceUnavailable
	}
	return m.detector.DetectPerson(frame)
}

func (m *Manager) rememberPersonFrame(cameraID string, frame []byte, width, height int, confidence float32) {
	jpegBytes, err := encodeBGRJPEG(frame, width, height)
	if err != nil {
		m.setLastError(err)
		return
	}
	m.mu.Lock()
	m.personFrames[cameraID] = PersonFrame{JPEG: jpegBytes, DetectedAt: time.Now().UTC(), Confidence: float64(confidence)}
	m.mu.Unlock()
}

func encodeBGRJPEG(frame []byte, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 || len(frame) != width*height*3 {
		return nil, errors.New("invalid BGR frame for JPEG")
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		src := i * 3
		dst := i * 4
		img.Pix[dst] = frame[src+2]
		img.Pix[dst+1] = frame[src+1]
		img.Pix[dst+2] = frame[src]
		img.Pix[dst+3] = 0xff
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, fmt.Errorf("encode person frame JPEG: %w", err)
	}
	return output.Bytes(), nil
}

func (m *Manager) liveLoop(ctx context.Context, source CameraSource) {
	defer m.wg.Done()
	stream := analyticsStreamURI(source.StreamURI)
	for ctx.Err() == nil {
		err := m.watchLive(ctx, source.ID, stream)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			m.setLastError(fmt.Errorf("live analytics %s: %w", source.ID, err))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(liveRestartDelay):
		}
	}
}

func (m *Manager) watchLive(ctx context.Context, cameraID, streamURI string) error {
	width, height := m.liveFrameDimensions(ctx, streamURI)
	filter := fmt.Sprintf("fps=1,scale=%d:%d,format=bgr24", width, height)
	cmd := exec.CommandContext(ctx, m.ffmpeg, "-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp", "-threads", "1", "-i", streamURI, "-an", "-vf", filter, "-pix_fmt", "bgr24", "-f", "rawvideo", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	frame := make([]byte, width*height*3)
	for {
		if _, err := io.ReadFull(stdout, frame); err != nil {
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("frame stream ended: %v: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		modelFrame := letterboxBGR(frame, width, height, InputWidth, InputHeight)
		confidence, found, err := m.detect(modelFrame)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		if !found {
			continue
		}
		m.rememberPersonFrame(cameraID, frame, width, height, confidence)
		status := m.media.Status(cameraID)
		if !status.Active || status.Path == "" || status.StartedAt.IsZero() {
			continue
		}
		offset := time.Since(status.StartedAt).Seconds()
		if offset < 0 {
			offset = 0
		}
		_ = m.store.UpsertLiveDetection(ctx, cameraID, filepath.Base(status.Path), offset, float64(confidence))
	}
}

func (m *Manager) liveFrameDimensions(ctx context.Context, streamURI string) (int, int) {
	if m.ffprobe == "" {
		return InputWidth, 180
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, m.ffprobe,
		"-v", "error",
		"-rtsp_transport", "tcp",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		streamURI,
	)
	output, err := cmd.Output()
	if err != nil {
		return InputWidth, 180
	}
	var sourceWidth, sourceHeight int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%dx%d", &sourceWidth, &sourceHeight); err != nil {
		return InputWidth, 180
	}
	return fitWithin(sourceWidth, sourceHeight, InputWidth, InputHeight)
}

func fitWithin(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= 0 || height <= 0 || maxWidth <= 0 || maxHeight <= 0 {
		return maxWidth, maxHeight
	}
	if width*maxHeight >= height*maxWidth {
		fittedHeight := (height*maxWidth + width/2) / width
		if fittedHeight < 1 {
			fittedHeight = 1
		}
		return maxWidth, fittedHeight
	}
	fittedWidth := (width*maxHeight + height/2) / height
	if fittedWidth < 1 {
		fittedWidth = 1
	}
	return fittedWidth, maxHeight
}

func letterboxBGR(frame []byte, width, height, targetWidth, targetHeight int) []byte {
	if width <= 0 || height <= 0 || targetWidth <= 0 || targetHeight <= 0 || width > targetWidth || height > targetHeight || len(frame) != width*height*3 {
		return nil
	}
	output := make([]byte, targetWidth*targetHeight*3)
	offsetX := (targetWidth - width) / 2
	offsetY := (targetHeight - height) / 2
	rowBytes := width * 3
	for y := 0; y < height; y++ {
		srcStart := y * rowBytes
		dstStart := ((y+offsetY)*targetWidth + offsetX) * 3
		copy(output[dstStart:dstStart+rowBytes], frame[srcStart:srcStart+rowBytes])
	}
	return output
}

func analyticsStreamURI(streamURI string) string {
	parsed, err := url.Parse(streamURI)
	if err != nil {
		return streamURI
	}
	query := parsed.Query()
	if query.Get("subtype") == "0" {
		query.Set("subtype", "1")
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func (m *Manager) failJob(job analysisJob, err error) {
	if _, statErr := os.Stat(job.Path); errors.Is(statErr, os.ErrNotExist) {
		_ = m.store.DeleteRecording(context.Background(), job.CameraID, job.RecordingName)
		m.clearAnalyzing()
		return
	}
	_ = m.store.Fail(context.Background(), job.CameraID, job.RecordingName, ModelName, compactError(err))
	m.setLastError(err)
	m.clearAnalyzing()
}
func (m *Manager) setLastError(err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	m.status.LastError = compactError(err)
	m.mu.Unlock()
}
func (m *Manager) setAnalyzing(job analysisJob, progress float64) {
	m.mu.Lock()
	m.status.Analyzing = true
	m.status.CameraID = job.CameraID
	m.status.RecordingName = job.RecordingName
	m.status.ProgressSecond = progress
	m.status.DurationSecond = job.Duration
	m.mu.Unlock()
}
func (m *Manager) clearAnalyzing() {
	m.mu.Lock()
	m.status.Analyzing = false
	m.status.CameraID = ""
	m.status.RecordingName = ""
	m.status.ProgressSecond = 0
	m.status.DurationSecond = 0
	m.mu.Unlock()
}
func compactError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if len(text) > 320 {
		text = text[:320]
	}
	return text
}
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
