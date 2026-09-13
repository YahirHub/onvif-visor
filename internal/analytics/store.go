package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type RecordingAnalysis struct {
	CameraID       string    `json:"camera_id"`
	RecordingName  string    `json:"recording_name"`
	SizeBytes      int64     `json:"size_bytes"`
	ModifiedUnixNS int64     `json:"modified_unix_ns"`
	DurationSecond float64   `json:"duration_seconds"`
	Status         string    `json:"status"`
	ProgressSecond float64   `json:"progress_seconds"`
	Model          string    `json:"model"`
	AnalyzedAt     time.Time `json:"analyzed_at,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type PersonSegment struct {
	ID            int64   `json:"id"`
	CameraID      string  `json:"camera_id"`
	RecordingName string  `json:"recording_name"`
	StartSecond   float64 `json:"start_seconds"`
	EndSecond     float64 `json:"end_seconds"`
	PeakSecond    float64 `json:"peak_seconds"`
	MaxConfidence float64 `json:"max_confidence"`
	Source        string  `json:"source"`
}

func OpenStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create analytics data dir: %w", err)
	}
	path := filepath.Join(dataDir, "analytics.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open analytics sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure analytics sqlite: %w", err)
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS recording_analysis (
    camera_id TEXT NOT NULL,
    recording_name TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    modified_unix_ns INTEGER NOT NULL DEFAULT 0,
    duration_seconds REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    progress_seconds REAL NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    analyzed_at INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(camera_id, recording_name)
);
CREATE TABLE IF NOT EXISTS person_segments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    camera_id TEXT NOT NULL,
    recording_name TEXT NOT NULL,
    start_seconds REAL NOT NULL,
    end_seconds REAL NOT NULL,
    peak_seconds REAL NOT NULL,
    max_confidence REAL NOT NULL,
    source TEXT NOT NULL DEFAULT 'offline'
);
CREATE INDEX IF NOT EXISTS idx_person_segments_camera_recording
    ON person_segments(camera_id, recording_name, start_seconds);
CREATE INDEX IF NOT EXISTS idx_recording_analysis_status
    ON recording_analysis(status, analyzed_at);
`); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate analytics sqlite: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("chmod analytics sqlite: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) NeedsAnalysis(ctx context.Context, cameraID, recordingName string, sizeBytes, modifiedUnixNS int64, model string) (bool, error) {
	var size, modified int64
	var status, currentModel string
	err := s.db.QueryRowContext(ctx, `SELECT size_bytes, modified_unix_ns, status, model FROM recording_analysis WHERE camera_id=? AND recording_name=?`, cameraID, recordingName).Scan(&size, &modified, &status, &currentModel)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("lookup recording analysis: %w", err)
	}
	return status != "complete" || size != sizeBytes || modified != modifiedUnixNS || currentModel != model, nil
}

func (s *Store) MarkAnalyzing(ctx context.Context, cameraID, recordingName string, sizeBytes, modifiedUnixNS int64, duration float64, model string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO recording_analysis(camera_id, recording_name, size_bytes, modified_unix_ns, duration_seconds, status, progress_seconds, model, analyzed_at, error)
VALUES(?,?,?,?,?,'analyzing',0,?,0,'')
ON CONFLICT(camera_id, recording_name) DO UPDATE SET
 size_bytes=excluded.size_bytes, modified_unix_ns=excluded.modified_unix_ns,
 duration_seconds=excluded.duration_seconds, status='analyzing', progress_seconds=0,
 model=excluded.model, analyzed_at=0, error=''`, cameraID, recordingName, sizeBytes, modifiedUnixNS, duration, model)
	if err != nil {
		return fmt.Errorf("mark recording analyzing: %w", err)
	}
	return nil
}

func (s *Store) UpdateProgress(ctx context.Context, cameraID, recordingName string, progress float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE recording_analysis SET progress_seconds=? WHERE camera_id=? AND recording_name=?`, progress, cameraID, recordingName)
	return err
}

func (s *Store) Complete(ctx context.Context, analysis RecordingAnalysis, segments []PersonSegment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin analytics complete: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM person_segments WHERE camera_id=? AND recording_name=?`, analysis.CameraID, analysis.RecordingName); err != nil {
		return fmt.Errorf("replace analytics segments: %w", err)
	}
	for _, segment := range segments {
		if _, err := tx.ExecContext(ctx, `INSERT INTO person_segments(camera_id, recording_name, start_seconds, end_seconds, peak_seconds, max_confidence, source) VALUES(?,?,?,?,?,?,?)`,
			analysis.CameraID, analysis.RecordingName, segment.StartSecond, segment.EndSecond, segment.PeakSecond, segment.MaxConfidence, segment.Source); err != nil {
			return fmt.Errorf("insert analytics segment: %w", err)
		}
	}
	analyzedAt := analysis.AnalyzedAt
	if analyzedAt.IsZero() {
		analyzedAt = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO recording_analysis(camera_id, recording_name, size_bytes, modified_unix_ns, duration_seconds, status, progress_seconds, model, analyzed_at, error)
VALUES(?,?,?,?,?,'complete',?,?,?,'')
ON CONFLICT(camera_id, recording_name) DO UPDATE SET
 size_bytes=excluded.size_bytes, modified_unix_ns=excluded.modified_unix_ns,
 duration_seconds=excluded.duration_seconds, status='complete', progress_seconds=excluded.progress_seconds,
 model=excluded.model, analyzed_at=excluded.analyzed_at, error=''`, analysis.CameraID, analysis.RecordingName, analysis.SizeBytes, analysis.ModifiedUnixNS, analysis.DurationSecond, analysis.DurationSecond, analysis.Model, analyzedAt.Unix()); err != nil {
		return fmt.Errorf("complete recording analysis: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recording analysis: %w", err)
	}
	return nil
}

func (s *Store) Fail(ctx context.Context, cameraID, recordingName, model string, errText string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO recording_analysis(camera_id, recording_name, status, model, analyzed_at, error)
VALUES(?,?,'error',?,? ,?)
ON CONFLICT(camera_id, recording_name) DO UPDATE SET status='error', model=excluded.model, analyzed_at=excluded.analyzed_at, error=excluded.error`, cameraID, recordingName, model, time.Now().UTC().Unix(), errText)
	return err
}

func (s *Store) UpsertLiveDetection(ctx context.Context, cameraID, recordingName string, offset, confidence float64) error {
	const mergeGap = 3.0
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var end, peak, maxConfidence float64
	err = tx.QueryRowContext(ctx, `SELECT id, end_seconds, peak_seconds, max_confidence FROM person_segments WHERE camera_id=? AND recording_name=? ORDER BY end_seconds DESC LIMIT 1`, cameraID, recordingName).Scan(&id, &end, &peak, &maxConfidence)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	segmentEnd := offset + 1.0
	if err == nil && offset-end <= mergeGap && offset >= end-1.0 {
		if segmentEnd < end {
			segmentEnd = end
		}
		if confidence > maxConfidence {
			maxConfidence = confidence
			peak = offset
		}
		if _, err := tx.ExecContext(ctx, `UPDATE person_segments SET end_seconds=?, peak_seconds=?, max_confidence=?, source='live' WHERE id=?`, segmentEnd, peak, maxConfidence, id); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO person_segments(camera_id, recording_name, start_seconds, end_seconds, peak_seconds, max_confidence, source) VALUES(?,?,?,?,?,?,'live')`, cameraID, recordingName, offset, segmentEnd, offset, confidence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteRecording(ctx context.Context, cameraID, recordingName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin analytics recording delete: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM person_segments WHERE camera_id=? AND recording_name=?`, cameraID, recordingName); err != nil {
		return fmt.Errorf("delete person segments: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM recording_analysis WHERE camera_id=? AND recording_name=?`, cameraID, recordingName); err != nil {
		return fmt.Errorf("delete recording analysis: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit analytics recording delete: %w", err)
	}
	return nil
}

func (s *Store) Segments(ctx context.Context, cameraID string) ([]PersonSegment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, camera_id, recording_name, start_seconds, end_seconds, peak_seconds, max_confidence, source FROM person_segments WHERE camera_id=? ORDER BY recording_name DESC, start_seconds ASC`, cameraID)
	if err != nil {
		return nil, fmt.Errorf("query person segments: %w", err)
	}
	defer rows.Close()
	segments := make([]PersonSegment, 0)
	for rows.Next() {
		var segment PersonSegment
		if err := rows.Scan(&segment.ID, &segment.CameraID, &segment.RecordingName, &segment.StartSecond, &segment.EndSecond, &segment.PeakSecond, &segment.MaxConfidence, &segment.Source); err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	return segments, rows.Err()
}

func (s *Store) ResetAnalyses(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE recording_analysis SET status='pending', progress_seconds=0, error=''`)
	if err != nil {
		return fmt.Errorf("reset recording analyses: %w", err)
	}
	return nil
}

func (s *Store) Analyses(ctx context.Context, cameraID string) ([]RecordingAnalysis, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT camera_id, recording_name, size_bytes, modified_unix_ns, duration_seconds, status, progress_seconds, model, analyzed_at, error FROM recording_analysis WHERE camera_id=? ORDER BY recording_name DESC`, cameraID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecordingAnalysis, 0)
	for rows.Next() {
		var item RecordingAnalysis
		var analyzedUnix int64
		if err := rows.Scan(&item.CameraID, &item.RecordingName, &item.SizeBytes, &item.ModifiedUnixNS, &item.DurationSecond, &item.Status, &item.ProgressSecond, &item.Model, &analyzedUnix, &item.Error); err != nil {
			return nil, err
		}
		if analyzedUnix > 0 {
			item.AnalyzedAt = time.Unix(analyzedUnix, 0).UTC()
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
