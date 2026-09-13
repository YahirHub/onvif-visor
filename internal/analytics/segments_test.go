package analytics

import "testing"

func TestMergeDetectionPoints(t *testing.T) {
	segments := MergeDetectionPoints([]DetectionPoint{
		{Second: 1, Confidence: .6, BestScore: .5},
		{Second: 2, Confidence: .8, BestScore: .9},
		{Second: 5, Confidence: .7, BestScore: .7},
		{Second: 12, Confidence: .9, BestScore: .8},
	}, 3)
	if len(segments) != 2 {
		t.Fatalf("segments=%#v", segments)
	}
	if segments[0].StartSecond != 1 || segments[0].EndSecond != 6 || segments[0].PeakSecond != 2 || segments[0].MaxConfidence != .8 || segments[0].BestScore != .9 {
		t.Fatalf("first segment=%#v", segments[0])
	}
	if segments[1].StartSecond != 12 || segments[1].EndSecond != 13 {
		t.Fatalf("second segment=%#v", segments[1])
	}
}

func TestMergeDetectionPointsKeepsConfidenceSeparateFromBestFrame(t *testing.T) {
	segments := MergeDetectionPoints([]DetectionPoint{
		{Second: 1, Confidence: .95, BestScore: .45},
		{Second: 2, Confidence: .80, BestScore: .88},
	}, 3)
	if len(segments) != 1 {
		t.Fatalf("segments=%#v", segments)
	}
	if segments[0].MaxConfidence != .95 || segments[0].PeakSecond != 2 || segments[0].BestScore != .88 {
		t.Fatalf("segment=%#v", segments[0])
	}
}
