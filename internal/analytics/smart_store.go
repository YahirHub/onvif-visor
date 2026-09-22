package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func migrateSmartSchema(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS object_segments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    camera_id TEXT NOT NULL,
    recording_name TEXT NOT NULL,
    class_id INTEGER NOT NULL,
    class_name TEXT NOT NULL,
    start_seconds REAL NOT NULL,
    end_seconds REAL NOT NULL,
    peak_seconds REAL NOT NULL,
    max_confidence REAL NOT NULL,
    best_score REAL NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'offline'
);
CREATE INDEX IF NOT EXISTS idx_object_segments_camera_recording
    ON object_segments(camera_id, recording_name, class_name, start_seconds);
CREATE INDEX IF NOT EXISTS idx_object_segments_camera_class
    ON object_segments(camera_id, class_name, recording_name);
CREATE TABLE IF NOT EXISTS analytics_config (
    camera_id TEXT PRIMARY KEY,
    config_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS smart_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    camera_id TEXT NOT NULL,
    recording_name TEXT NOT NULL DEFAULT '',
    offset_seconds REAL NOT NULL DEFAULT 0,
    occurred_at INTEGER NOT NULL,
    class_id INTEGER NOT NULL,
    class_name TEXT NOT NULL,
    track_id INTEGER NOT NULL DEFAULT 0,
    event_type TEXT NOT NULL,
    zone_id TEXT NOT NULL DEFAULT '',
    zone_name TEXT NOT NULL DEFAULT '',
    line_id TEXT NOT NULL DEFAULT '',
    line_name TEXT NOT NULL DEFAULT '',
    direction TEXT NOT NULL DEFAULT '',
    confidence REAL NOT NULL DEFAULT 0,
    best_score REAL NOT NULL DEFAULT 0,
    matched_rule_ids TEXT NOT NULL DEFAULT '[]',
    matched_rule_names TEXT NOT NULL DEFAULT '[]',
    notify_telegram INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_smart_events_camera_time
    ON smart_events(camera_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_smart_events_recording
    ON smart_events(camera_id, recording_name, offset_seconds);
`); err != nil {
		return fmt.Errorf("migrate smart analytics sqlite: %w", err)
	}
	var objectCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM object_segments`).Scan(&objectCount); err != nil {
		return fmt.Errorf("count object segments: %w", err)
	}
	if objectCount == 0 {
		if _, err := db.Exec(`
INSERT INTO object_segments(camera_id, recording_name, class_id, class_name, start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source)
SELECT camera_id, recording_name, 0, 'person', start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source
FROM person_segments`); err != nil {
			return fmt.Errorf("migrate legacy person segments: %w", err)
		}
	}
	return nil
}

func (s *Store) CameraConfig(ctx context.Context, cameraID string) (CameraAnalyticsConfig, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT config_json FROM analytics_config WHERE camera_id=?`, cameraID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultCameraAnalyticsConfig(), nil
	}
	if err != nil {
		return CameraAnalyticsConfig{}, fmt.Errorf("load analytics config: %w", err)
	}
	var cfg CameraAnalyticsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return CameraAnalyticsConfig{}, fmt.Errorf("decode analytics config: %w", err)
	}
	return normalizeAnalyticsConfig(cfg)
}

func (s *Store) SaveCameraConfig(ctx context.Context, cameraID string, cfg CameraAnalyticsConfig) (CameraAnalyticsConfig, error) {
	clean, err := normalizeAnalyticsConfig(cfg)
	if err != nil {
		return CameraAnalyticsConfig{}, err
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return CameraAnalyticsConfig{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO analytics_config(camera_id, config_json, updated_at) VALUES(?,?,?)
ON CONFLICT(camera_id) DO UPDATE SET config_json=excluded.config_json, updated_at=excluded.updated_at`, cameraID, string(raw), time.Now().UTC().Unix()); err != nil {
		return CameraAnalyticsConfig{}, fmt.Errorf("save analytics config: %w", err)
	}
	return clean, nil
}

func (s *Store) DeleteCameraConfig(ctx context.Context, cameraID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM analytics_config WHERE camera_id=?`, cameraID)
	return err
}

func (s *Store) ObjectSegments(ctx context.Context, cameraID string) ([]ObjectSegment, error) {
	return s.queryObjectSegments(ctx, `
SELECT id, camera_id, recording_name, class_id, class_name, start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source
FROM object_segments WHERE camera_id=? ORDER BY recording_name DESC, start_seconds ASC, class_id ASC`, cameraID)
}

func (s *Store) ObjectSegmentsForRecording(ctx context.Context, cameraID, recordingName string) ([]ObjectSegment, error) {
	return s.queryObjectSegments(ctx, `
SELECT id, camera_id, recording_name, class_id, class_name, start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source
FROM object_segments WHERE camera_id=? AND recording_name=? ORDER BY start_seconds ASC, class_id ASC`, cameraID, recordingName)
}

func (s *Store) queryObjectSegments(ctx context.Context, query string, args ...any) ([]ObjectSegment, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query object segments: %w", err)
	}
	defer rows.Close()
	out := make([]ObjectSegment, 0)
	for rows.Next() {
		var segment ObjectSegment
		if err := rows.Scan(&segment.ID, &segment.CameraID, &segment.RecordingName, &segment.ClassID, &segment.ClassName, &segment.StartSecond, &segment.EndSecond, &segment.PeakSecond, &segment.MaxConfidence, &segment.BestScore, &segment.Source); err != nil {
			return nil, err
		}
		out = append(out, segment)
	}
	return out, rows.Err()
}

func (s *Store) CompleteObjects(ctx context.Context, analysis RecordingAnalysis, segments []ObjectSegment, events []SmartEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin object analytics complete: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM object_segments WHERE camera_id=? AND recording_name=?`, analysis.CameraID, analysis.RecordingName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM smart_events WHERE camera_id=? AND recording_name=?`, analysis.CameraID, analysis.RecordingName); err != nil {
		return err
	}
	for _, segment := range segments {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO object_segments(camera_id, recording_name, class_id, class_name, start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source)
VALUES(?,?,?,?,?,?,?,?,?,?)`, analysis.CameraID, analysis.RecordingName, segment.ClassID, segment.ClassName, segment.StartSecond, segment.EndSecond, segment.PeakSecond, segment.MaxConfidence, segment.BestScore, segment.Source); err != nil {
			return fmt.Errorf("insert object segment: %w", err)
		}
	}
	for _, event := range events {
		if err := insertSmartEventTx(ctx, tx, event); err != nil {
			return err
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
		return fmt.Errorf("complete object recording analysis: %w", err)
	}
	return tx.Commit()
}

func (s *Store) UpsertLiveObjectDetection(ctx context.Context, cameraID, recordingName string, classID int, className string, offset, confidence, bestScore float64) error {
	const mergeGap = 3.0
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var end, peak, maxConfidence, currentBest float64
	err = tx.QueryRowContext(ctx, `
SELECT id, end_seconds, peak_seconds, max_confidence, best_score
FROM object_segments
WHERE camera_id=? AND recording_name=? AND class_name=?
ORDER BY end_seconds DESC LIMIT 1`, cameraID, recordingName, className).Scan(&id, &end, &peak, &maxConfidence, &currentBest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	segmentEnd := offset + 1
	if err == nil && offset-end <= mergeGap && offset >= end-1 {
		if segmentEnd < end {
			segmentEnd = end
		}
		if confidence > maxConfidence {
			maxConfidence = confidence
		}
		if bestScore > currentBest {
			currentBest, peak = bestScore, offset
		}
		if _, err := tx.ExecContext(ctx, `UPDATE object_segments SET end_seconds=?, peak_seconds=?, max_confidence=?, best_score=?, source='live' WHERE id=?`, segmentEnd, peak, maxConfidence, currentBest, id); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO object_segments(camera_id, recording_name, class_id, class_name, start_seconds, end_seconds, peak_seconds, max_confidence, best_score, source)
VALUES(?,?,?,?,?,?,?,?,?,'live')`, cameraID, recordingName, classID, className, offset, segmentEnd, offset, confidence, bestScore); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) InsertSmartEvents(ctx context.Context, events []SmartEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, event := range events {
		if err := insertSmartEventTx(ctx, tx, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertSmartEventTx(ctx context.Context, tx *sql.Tx, event SmartEvent) error {
	ruleIDs, _ := json.Marshal(event.MatchedRuleIDs)
	ruleNames, _ := json.Marshal(event.MatchedRuleNames)
	occurred := event.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO smart_events(camera_id, recording_name, offset_seconds, occurred_at, class_id, class_name, track_id, event_type, zone_id, zone_name, line_id, line_name, direction, confidence, best_score, matched_rule_ids, matched_rule_names, notify_telegram)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, event.CameraID, event.RecordingName, event.OffsetSecond, occurred.UnixMilli(), event.ClassID, event.ClassName, event.TrackID, event.Type, event.ZoneID, event.ZoneName, event.LineID, event.LineName, event.Direction, event.Confidence, event.BestScore, string(ruleIDs), string(ruleNames), boolInt(event.NotifyTelegram))
	if err != nil {
		return fmt.Errorf("insert smart event: %w", err)
	}
	return nil
}

func (s *Store) SmartEvents(ctx context.Context, cameraID string, from, to time.Time) ([]SmartEvent, error) {
	query := `SELECT id, camera_id, recording_name, offset_seconds, occurred_at, class_id, class_name, track_id, event_type, zone_id, zone_name, line_id, line_name, direction, confidence, best_score, matched_rule_ids, matched_rule_names, notify_telegram FROM smart_events WHERE camera_id=?`
	args := []any{cameraID}
	if !from.IsZero() {
		query += ` AND occurred_at>=?`
		args = append(args, from.UnixMilli())
	}
	if !to.IsZero() {
		query += ` AND occurred_at<?`
		args = append(args, to.UnixMilli())
	}
	query += ` ORDER BY occurred_at DESC, id DESC LIMIT 5000`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query smart events: %w", err)
	}
	defer rows.Close()
	out := make([]SmartEvent, 0)
	for rows.Next() {
		var event SmartEvent
		var occurredMS int64
		var ruleIDs, ruleNames string
		var notify int
		if err := rows.Scan(&event.ID, &event.CameraID, &event.RecordingName, &event.OffsetSecond, &occurredMS, &event.ClassID, &event.ClassName, &event.TrackID, &event.Type, &event.ZoneID, &event.ZoneName, &event.LineID, &event.LineName, &event.Direction, &event.Confidence, &event.BestScore, &ruleIDs, &ruleNames, &notify); err != nil {
			return nil, err
		}
		event.OccurredAt = time.UnixMilli(occurredMS).UTC()
		_ = json.Unmarshal([]byte(ruleIDs), &event.MatchedRuleIDs)
		_ = json.Unmarshal([]byte(ruleNames), &event.MatchedRuleNames)
		event.NotifyTelegram = notify != 0
		out = append(out, event)
	}
	return out, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
