package analytics

import (
	"context"
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
	needs, err := s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, ModelName)
	if err != nil || !needs {
		t.Fatalf("initial needs=%v err=%v", needs, err)
	}
	analysis := RecordingAnalysis{CameraID: "cam", RecordingName: "clip.mkv", SizeBytes: 100, ModifiedUnixNS: 20, DurationSecond: 10, Model: ModelName, AnalyzedAt: time.Now()}
	segments := []PersonSegment{{StartSecond: 2, EndSecond: 5, PeakSecond: 3, MaxConfidence: .8, Source: "offline"}}
	if err := s.Complete(ctx, analysis, segments); err != nil {
		t.Fatal(err)
	}
	needs, err = s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, ModelName)
	if err != nil || needs {
		t.Fatalf("same fingerprint needs=%v err=%v", needs, err)
	}
	needs, err = s.NeedsAnalysis(ctx, "cam", "clip.mkv", 101, 20, ModelName)
	if err != nil || !needs {
		t.Fatalf("changed fingerprint needs=%v err=%v", needs, err)
	}
	got, err := s.Segments(ctx, "cam")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PeakSecond != 3 {
		t.Fatalf("segments=%#v", got)
	}
}

func TestUpsertLiveDetectionMergesNearbyPoints(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range []struct{ s, c float64 }{{1, .5}, {2, .8}, {5, .7}, {12, .9}} {
		if err := s.UpsertLiveDetection(ctx, "cam", "clip.mkv", p.s, p.c); err != nil {
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
	if got[0].StartSecond != 1 || got[0].EndSecond != 6 || got[0].PeakSecond != 2 {
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
	analysis := RecordingAnalysis{CameraID: "cam", RecordingName: "clip.mkv", SizeBytes: 100, ModifiedUnixNS: 20, DurationSecond: 10, Model: ModelName}
	if err := s.Complete(ctx, analysis, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetAnalyses(ctx); err != nil {
		t.Fatal(err)
	}
	needs, err := s.NeedsAnalysis(ctx, "cam", "clip.mkv", 100, 20, ModelName)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Fatal("reset analysis should be pending for reindex")
	}
}
