package analytics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	model    string
	runtime  string
	media    *media.Manager
	store    *Store
	detector Detector

	inferMu sync.Mutex
	mu      sync.Mutex
	sources map[string]CameraSource
	live    map[string]context.CancelFunc
	queued  map[string]bool
	status  Status
	jobs    chan analysisJob
	wg      sync.WaitGroup
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
	m := &Manager{
		ctx: ctx, cancel: cancel, ffmpeg: cfg.FFmpeg, model: cfg.ModelPath,
		runtime: ResolveRuntimePath(cfg.RuntimePath), media: cfg.Media, store: store,
		sources: make(map[string]CameraSource), live: make(map[string]context.CancelFunc),
		queued: make(map[string]bool), jobs: make(chan analysisJob, 128),
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
	filter := fmt.Sprintf("fps=1,scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=bgr24", InputWidth, InputHeight, InputWidth, InputHeight)
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
	frame := make([]byte, frameSize)
	for {
		if _, err := io.ReadFull(stdout, frame); err != nil {
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("frame stream ended: %v: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		confidence, found, err := m.detect(frame)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		if !found {
			continue
		}
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
