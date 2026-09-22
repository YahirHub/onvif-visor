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
	frameSize                     = InputWidth * InputHeight * 3
	scanInterval                  = 15 * time.Second
	liveRestartDelay              = 3 * time.Second
	progressEveryFrame            = 10
	liveProgressInterval          = 10 * time.Second
	longRecordingSeconds          = 6 * 60 * 60
	liveProgressCoverageTolerance = 15
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
	StartedAt     time.Time
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
	configs      map[string]CameraAnalyticsConfig
	trackers     map[string]*Tracker
	eventHandler func(SmartEvent)
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
		queued: make(map[string]bool), personFrames: make(map[string]PersonFrame),
		configs: make(map[string]CameraAnalyticsConfig), trackers: make(map[string]*Tracker), jobs: make(chan analysisJob, 128),
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

func (m *Manager) ObjectSegments(ctx context.Context, cameraID string) ([]ObjectSegment, error) {
	return m.store.ObjectSegments(ctx, cameraID)
}

func (m *Manager) ObjectSegmentsForRecording(ctx context.Context, cameraID, recordingName string) ([]ObjectSegment, error) {
	return m.store.ObjectSegmentsForRecording(ctx, cameraID, recordingName)
}

func (m *Manager) SmartEvents(ctx context.Context, cameraID string, from, to time.Time) ([]SmartEvent, error) {
	return m.store.SmartEvents(ctx, cameraID, from, to)
}

func (m *Manager) CameraConfig(ctx context.Context, cameraID string) (CameraAnalyticsConfig, error) {
	cfg, err := m.store.CameraConfig(ctx, cameraID)
	if err != nil {
		return CameraAnalyticsConfig{}, err
	}
	m.mu.Lock()
	m.configs[cameraID] = cfg
	if m.trackers[cameraID] == nil {
		m.trackers[cameraID] = NewTracker()
	}
	m.mu.Unlock()
	return cfg, nil
}

func (m *Manager) SaveCameraConfig(ctx context.Context, cameraID string, cfg CameraAnalyticsConfig) (CameraAnalyticsConfig, error) {
	clean, err := m.store.SaveCameraConfig(ctx, cameraID, cfg)
	if err != nil {
		return CameraAnalyticsConfig{}, err
	}
	m.mu.Lock()
	m.configs[cameraID] = clean
	m.trackers[cameraID] = NewTracker()
	m.mu.Unlock()
	return clean, nil
}

func (m *Manager) SetEventHandler(handler func(SmartEvent)) {
	m.mu.Lock()
	m.eventHandler = handler
	m.mu.Unlock()
}

func (m *Manager) configAndTracker(cameraID string) (CameraAnalyticsConfig, *Tracker) {
	m.mu.Lock()
	cfg, ok := m.configs[cameraID]
	tracker := m.trackers[cameraID]
	m.mu.Unlock()
	if !ok {
		loaded, err := m.store.CameraConfig(m.ctx, cameraID)
		if err == nil {
			cfg = loaded
		} else {
			cfg = DefaultCameraAnalyticsConfig()
			m.setLastError(err)
		}
	}
	m.mu.Lock()
	if _, exists := m.configs[cameraID]; !exists {
		m.configs[cameraID] = cfg
	}
	if tracker == nil {
		tracker = NewTracker()
		m.trackers[cameraID] = tracker
	}
	m.mu.Unlock()
	return cfg, tracker
}

func (m *Manager) emitSmartEvent(event SmartEvent) {
	m.mu.Lock()
	handler := m.eventHandler
	m.mu.Unlock()
	if handler != nil {
		handler(event)
	}
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
		if !ok || frame.DetectedAt.Before(notBefore) {
			return PersonFrame{}, false
		}
		return frame, true
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
	delete(m.configs, cameraID)
	delete(m.trackers, cameraID)
	m.mu.Unlock()
	_ = m.store.DeleteCameraConfig(context.Background(), cameraID)
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

func (m *Manager) ReindexCamera(cameraID string) {
	go func() {
		if err := m.store.ResetCameraAnalyses(m.ctx, cameraID); err != nil {
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
				if file.DurationSecond >= longRecordingSeconds {
					progress, live, err := m.store.LiveProgress(m.ctx, source.ID, file.Name, AnalysisModelKey)
					if err != nil {
						m.setLastError(err)
						continue
					}
					if live && file.DurationSecond-progress <= liveProgressCoverageTolerance {
						analysis := RecordingAnalysis{
							CameraID: source.ID, RecordingName: file.Name,
							SizeBytes: info.Size(), ModifiedUnixNS: info.ModTime().UnixNano(),
							DurationSecond: file.DurationSecond, Model: AnalysisModelKey,
							AnalyzedAt: time.Now().UTC(),
						}
						if err := m.store.CompleteLiveProgress(m.ctx, analysis); err != nil {
							m.setLastError(err)
						} else {
							continue
						}
					}
				}
				needs, err := m.store.NeedsAnalysis(m.ctx, source.ID, file.Name, info.Size(), info.ModTime().UnixNano(), AnalysisModelKey)
				if err != nil {
					m.setLastError(err)
					continue
				}
				if !needs {
					continue
				}
			}
			job := analysisJob{CameraID: source.ID, RecordingName: file.Name, Path: path, SizeBytes: info.Size(), ModifiedNS: info.ModTime().UnixNano(), Duration: file.DurationSecond, StartedAt: file.StartedAt}
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
	if err := m.store.MarkAnalyzing(m.ctx, job.CameraID, job.RecordingName, job.SizeBytes, job.ModifiedNS, job.Duration, AnalysisModelKey); err != nil {
		m.setLastError(err)
		return
	}
	m.setAnalyzing(job, 0)
	cfg, _ := m.configAndTracker(job.CameraID)
	enabled := enabledClassSet(cfg)
	tracker := NewTracker()
	points := make([]ObjectDetectionPoint, 0)
	events := make([]SmartEvent, 0)
	recordingStartedAt := job.StartedAt
	if recordingStartedAt.IsZero() {
		if job.ModifiedNS > 0 {
			recordingStartedAt = time.Unix(0, job.ModifiedNS).Add(-time.Duration(maxFloat(job.Duration, 1) * float64(time.Second)))
		} else {
			recordingStartedAt = time.Now().UTC().Add(-time.Duration(maxFloat(job.Duration, 1) * float64(time.Second)))
		}
	}
	width, height := m.recordingFrameDimensions(m.ctx, job.Path)
	filter := fmt.Sprintf("fps=1,scale=%d:%d,format=bgr24", width, height)
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
	frame := make([]byte, width*height*3)
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
		modelFrame := letterboxBGR(frame, width, height, InputWidth, InputHeight)
		detections, detectErr := m.detect(modelFrame)
		if detectErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			m.failJob(job, detectErr)
			return
		}
		detections = scoreDetections(modelFrame, detections, enabled)
		second := float64(frameIndex)
		points = append(points, pointsForFrame(second, detections, enabled)...)
		occurredAt := recordingStartedAt.Add(time.Duration(second * float64(time.Second)))
		events = append(events, tracker.Process(job.CameraID, job.RecordingName, second, occurredAt, detections, width, height, cfg)...)
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
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(job.Path); errors.Is(err, os.ErrNotExist) {
		_ = m.store.DeleteRecording(context.Background(), job.CameraID, job.RecordingName)
		m.clearAnalyzing()
		return
	}
	segments := MergeObjectDetectionPoints(points, 3)
	analysis := RecordingAnalysis{CameraID: job.CameraID, RecordingName: job.RecordingName, SizeBytes: job.SizeBytes, ModifiedUnixNS: job.ModifiedNS, DurationSecond: maxFloat(job.Duration, float64(frameIndex)), Status: "complete", ProgressSecond: float64(frameIndex), Model: AnalysisModelKey, AnalyzedAt: time.Now().UTC()}
	if err := m.store.CompleteObjects(m.ctx, analysis, segments, events); err != nil {
		m.setLastError(err)
	}
	m.clearAnalyzing()
}

func (m *Manager) detect(frame []byte) ([]Detection, error) {
	m.inferMu.Lock()
	defer m.inferMu.Unlock()
	if m.detector == nil {
		return nil, ErrInferenceUnavailable
	}
	return m.detector.Detect(frame)
}

func (m *Manager) rememberPersonFrame(cameraID string, confidence float32) {
	m.mu.Lock()
	m.personFrames[cameraID] = PersonFrame{DetectedAt: time.Now().UTC(), Confidence: float64(confidence)}
	m.mu.Unlock()
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
	var lastProgressAt time.Time
	lastProgressRecording := ""
	for {
		if _, err := io.ReadFull(stdout, frame); err != nil {
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("frame stream ended: %v: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
		modelFrame := letterboxBGR(frame, width, height, InputWidth, InputHeight)
		detections, err := m.detect(modelFrame)
		if err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		cfg, tracker := m.configAndTracker(cameraID)
		enabled := enabledClassSet(cfg)
		detections = scoreDetections(modelFrame, detections, enabled)
		if confidence, _, found := summarizeClassDetections(modelFrame, detections, 0); found {
			m.rememberPersonFrame(cameraID, float32(confidence))
		}

		now := time.Now().UTC()
		status := m.media.Status(cameraID)
		recordingName := ""
		offset := 0.0
		if status.Active && status.Path != "" && !status.StartedAt.IsZero() {
			recordingName = filepath.Base(status.Path)
			offset = now.Sub(status.StartedAt).Seconds()
			if offset < 0 {
				offset = 0
			}
			if recordingName != lastProgressRecording || lastProgressAt.IsZero() || now.Sub(lastProgressAt) >= liveProgressInterval {
				if err := m.store.MarkLiveProgress(ctx, cameraID, recordingName, offset, AnalysisModelKey); err != nil {
					m.setLastError(err)
				} else {
					lastProgressRecording = recordingName
					lastProgressAt = now
				}
			}
			for _, point := range pointsForFrame(offset, detections, enabled) {
				_ = m.store.UpsertLiveObjectDetection(ctx, cameraID, recordingName, point.ClassID, point.ClassName, offset, point.Confidence, point.BestScore)
			}
		}
		events := tracker.Process(cameraID, recordingName, offset, now, detections, width, height, cfg)
		if len(events) > 0 {
			if err := m.store.InsertSmartEvents(ctx, events); err != nil {
				m.setLastError(err)
			}
			for _, event := range events {
				m.emitSmartEvent(event)
			}
		}
	}
}

func (m *Manager) recordingFrameDimensions(ctx context.Context, path string) (int, int) {
	if m.ffprobe == "" {
		return InputWidth, 180
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, m.ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		path,
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
	_ = m.store.Fail(context.Background(), job.CameraID, job.RecordingName, AnalysisModelKey, compactError(err))
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
