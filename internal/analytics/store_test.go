package analytics

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCompleteAndFingerprint(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	needs, err := s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, AnalysisModelKey)
	if err != nil || !needs {
		t.Fatalf("initial needs=%v err=%v", needs, err)
	}
	analysis := RecordingAnalysis{CameraID: "cam", RecordingName: "clip.mkv", SizeBytes: 100, ModifiedUnixNS: 20, DurationSecond: 10, Model: AnalysisModelKey, AnalyzedAt: time.Now()}
	segments := []ObjectSegment{{ClassID: 0, ClassName: "person", StartSecond: 2, EndSecond: 5, PeakSecond: 3, MaxConfidence: .8, BestScore: .9, Source: "offline"}}
	if err := s.CompleteObjects(ctx, analysis, segments, nil); err != nil {
		t.Fatal(err)
	}
	needs, err = s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, AnalysisModelKey)
	if err != nil || needs {
		t.Fatalf("same fingerprint needs=%v err=%v", needs, err)
	}
	needs, err = s.NeedsAnalysis(ctx, "cam", "clip.mkv", 101, 20, AnalysisModelKey)
	if err != nil || !needs {
		t.Fatalf("changed fingerprint needs=%v err=%v", needs, err)
	}
	got, err := s.Segments(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PeakSecond != 3 || got[0].BestScore != .9 {
		t.Fatalf("segments=%#v", got)
	}
}

func TestRecordingPersonStatesOnlyTrustsCompletedCurrentAnalysis(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	withPerson := RecordingAnalysis{
		CameraID: "cam", RecordingName: "with-person.mkv",
		SizeBytes: 10, ModifiedUnixNS: 1, DurationSecond: 12,
		Model: AnalysisModelKey, AnalyzedAt: time.Now(),
	}
	if err := s.CompleteObjects(ctx, withPerson, []ObjectSegment{{
		ClassID: 0, ClassName: "person", StartSecond: 1, EndSecond: 4,
		PeakSecond: 2, MaxConfidence: .8, BestScore: .9, Source: "offline",
	}}, nil); err != nil {
		t.Fatal(err)
	}

	withoutPerson := RecordingAnalysis{
		CameraID: "cam", RecordingName: "without-person.mkv",
		SizeBytes: 11, ModifiedUnixNS: 2, DurationSecond: 10,
		Model: AnalysisModelKey, AnalyzedAt: time.Now(),
	}
	if err := s.CompleteObjects(ctx, withoutPerson, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAnalyzing(ctx, "cam", "pending.mkv", 12, 3, 10, AnalysisModelKey); err != nil {
		t.Fatal(err)
	}

	complete, persons, err := s.RecordingPersonStates(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if !complete["with-person.mkv"] || !complete["without-person.mkv"] {
		t.Fatalf("complete index = %#v", complete)
	}
	if complete["pending.mkv"] {
		t.Fatalf("pending recording was treated as complete: %#v", complete)
	}
	if !persons["with-person.mkv"] || persons["without-person.mkv"] {
		t.Fatalf("person index = %#v", persons)
	}
}

func TestLiveProgressCanBeCompletedWithoutDroppingSegments(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.MarkLiveProgress(ctx, "cam", "day.mkv", 3590, AnalysisModelKey); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertLiveObjectDetection(ctx, "cam", "day.mkv", 0, "person", 120, .82, .9); err != nil {
		t.Fatal(err)
	}
	progress, live, err := s.LiveProgress(ctx, "cam", "day.mkv", AnalysisModelKey)
	if err != nil || !live || progress != 3590 {
		t.Fatalf("live progress=%f live=%v err=%v", progress, live, err)
	}
	analysis := RecordingAnalysis{
		CameraID: "cam", RecordingName: "day.mkv", SizeBytes: 9000,
		ModifiedUnixNS: 123, DurationSecond: 3600, Model: AnalysisModelKey,
	}
	if err := s.CompleteLiveProgress(ctx, analysis); err != nil {
		t.Fatal(err)
	}
	needs, err := s.NeedsAnalysis(ctx, "cam", "day.mkv", 9000, 123, AnalysisModelKey)
	if err != nil || needs {
		t.Fatalf("completed live recording needs=%v err=%v", needs, err)
	}
	segments, err := s.ObjectSegmentsForRecording(ctx, "cam", "day.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].ClassName != "person" || segments[0].PeakSecond != 120 {
		t.Fatalf("segments=%#v", segments)
	}
}

func TestObjectSegmentsForRecordingFiltersOtherFiles(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, name := range []string{"one.mkv", "two.mkv"} {
		if err := s.UpsertLiveObjectDetection(ctx, "cam", name, 0, "person", 5, .7, .8); err != nil {
			t.Fatal(err)
		}
	}
	segments, err := s.ObjectSegmentsForRecording(ctx, "cam", "two.mkv")
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].RecordingName != "two.mkv" {
		t.Fatalf("filtered segments=%#v", segments)
	}
}

func TestUpsertLiveObjectDetectionMergesNearbyPoints(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range []struct{ s, c, best float64 }{{1, .5, .4}, {2, .8, .9}, {5, .7, .7}, {12, .9, .8}} {
		if err := s.UpsertLiveObjectDetection(ctx, "cam", "clip.mkv", 0, "person", p.s, p.c, p.best); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Segments(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("segments=%#v", got)
	}
	if got[0].StartSecond != 1 || got[0].EndSecond != 6 || got[0].PeakSecond != 2 || got[0].BestScore != .9 {
		t.Fatalf("first=%#v", got[0])
	}
}

func TestResetAnalysesMarksCompletePending(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	analysis := RecordingAnalysis{CameraID: "cam", RecordingName: "clip.mkv", SizeBytes: 100, ModifiedUnixNS: 20, DurationSecond: 10, Model: AnalysisModelKey}
	if err := s.CompleteObjects(ctx, analysis, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetAnalyses(ctx); err != nil {
		t.Fatal(err)
	}
	needs, err := s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, AnalysisModelKey)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("reset analysis should be pending for reindex")
	}
}

func TestDeleteRecordingRemovesAnalysisSegmentsAndEvents(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	analysis := RecordingAnalysis{CameraID: "cam", RecordingName: "clip.mkv", SizeBytes: 100, ModifiedUnixNS: 20, DurationSecond: 10, Model: AnalysisModelKey}
	segments := []ObjectSegment{{ClassID: 0, ClassName: "person", StartSecond: 1, EndSecond: 4, PeakSecond: 2, MaxConfidence: .9, BestScore: .8, Source: "offline"}}
	events := []SmartEvent{{CameraID: "cam", RecordingName: "clip.mkv", OccurredAt: time.Now(), ClassID: 0, ClassName: "person", Type: "object"}}
	if err := s.CompleteObjects(ctx, analysis, segments, events); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRecording(ctx, "cam", "clip.mkv"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ObjectSegments(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("segments after delete = %#v", got)
	}
	gotEvents, err := s.SmartEvents(ctx, "cam", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotEvents) != 0 {
		t.Fatalf("events after delete = %#v", gotEvents)
	}
	needs, err := s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, AnalysisModelKey)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("deleted analysis should no longer exist")
	}
}

func TestCameraAnalyticsConfigRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg := CameraAnalyticsConfig{
		EnabledClasses: []string{"person", "car"},
		Zones:          []Zone{{Name: "Entrada", Points: []Point{{0, 0}, {1, 0}, {1, 1}}}},
	}
	saved, err := s.SaveCameraConfig(ctx, "cam", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Zones[0].ID == "" {
		t.Fatal("expected generated zone id")
	}
	loaded, err := s.CameraConfig(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.EnabledClasses) != 2 || loaded.Zones[0].Name != "Entrada" {
		t.Fatalf("loaded=%#v", loaded)
	}
}

func TestOpenStoreMigratesLegacyPersonSegmentsToObjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE person_segments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		camera_id TEXT NOT NULL,
		recording_name TEXT NOT NULL,
		start_seconds REAL NOT NULL,
		end_seconds REAL NOT NULL,
		peak_seconds REAL NOT NULL,
		max_confidence REAL NOT NULL,
		source TEXT NOT NULL DEFAULT 'offline'
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO person_segments(camera_id, recording_name, start_seconds, end_seconds, peak_seconds, max_confidence, source) VALUES('cam','old.mkv',1,4,2,.88,'offline')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	segments, err := store.ObjectSegments(context.Background(), "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 1 || segments[0].ClassName != "person" || segments[0].PeakSecond != 2 || segments[0].MaxConfidence != .88 {
		t.Fatalf("migrated segments=%#v", segments)
	}
	if segments[0].BestScore != 0 {
		t.Fatalf("legacy best score=%f, want 0", segments[0].BestScore)
	}
}
