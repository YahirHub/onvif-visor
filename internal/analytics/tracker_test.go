package analytics

import (
	"testing"
	"time"
)

func testDetection(classID int, x1, y1, x2, y2 float32) Detection {
	return Detection{ClassID: classID, ClassName: ClassName(classID), Score: .85, BestScore: .8, Box: Box{X1: x1, Y1: y1, X2: x2, Y2: y2, Score: .85}}
}

func TestNormalizeDetectionsRemovesLetterbox(t *testing.T) {
	// 16:9 source is represented as 320x180 centered in a 320x320 tensor.
	got := normalizeDetections([]Detection{testDetection(0, 80, 115, 240, 205)}, 320, 180)
	if len(got) != 1 {
		t.Fatalf("normalized=%#v", got)
	}
	if got[0].Center.X < .49 || got[0].Center.X > .51 || got[0].Center.Y < .49 || got[0].Center.Y > .51 {
		t.Fatalf("center=%#v, want ~0.5/0.5", got[0].Center)
	}
}

func TestPointInPolygon(t *testing.T) {
	polygon := []Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	if !pointInPolygon(Point{.5, .5}, polygon) || pointInPolygon(Point{1.2, .5}, polygon) {
		t.Fatal("pointInPolygon returned unexpected result")
	}
}

func TestTrackerZoneEnterAndExit(t *testing.T) {
	tracker := NewTracker()
	cfg := DefaultCameraAnalyticsConfig()
	cfg.Zones = []Zone{{ID: "z1", Name: "Entrada", Points: []Point{{.4, .2}, {.9, .2}, {.9, .9}, {.4, .9}}}}
	now := time.Now().UTC()
	first := testDetection(0, 16, 100, 80, 220) // center ~.15 after normalization: outside
	if events := tracker.Process("cam", "clip.mkv", 1, now, []Detection{first}, 320, 320, cfg); len(events) != 1 || events[0].Type != "object" {
		t.Fatalf("first events=%#v", events)
	}
	inside := testDetection(0, 128, 100, 224, 220)
	events := tracker.Process("cam", "clip.mkv", 2, now.Add(time.Second), []Detection{inside}, 320, 320, cfg)
	foundEnter := false
	for _, event := range events {
		if event.Type == "zone_enter" && event.ZoneID == "z1" {
			foundEnter = true
		}
	}
	if !foundEnter {
		t.Fatalf("zone enter missing: %#v", events)
	}
	outside := testDetection(0, 8, 100, 72, 220)
	events = tracker.Process("cam", "clip.mkv", 3, now.Add(2*time.Second), []Detection{outside}, 320, 320, cfg)
	foundExit := false
	for _, event := range events {
		if event.Type == "zone_exit" && event.ZoneID == "z1" {
			foundExit = true
		}
	}
	if !foundExit {
		t.Fatalf("zone exit missing: %#v", events)
	}
}

func TestTrackerLineCrossDirection(t *testing.T) {
	tracker := NewTracker()
	cfg := DefaultCameraAnalyticsConfig()
	cfg.Lines = []Line{{ID: "l1", Name: "Puerta", A: Point{.5, 0}, B: Point{.5, 1}}}
	now := time.Now().UTC()
	left := testDetection(0, 64, 100, 128, 220)
	_ = tracker.Process("cam", "clip.mkv", 1, now, []Detection{left}, 320, 320, cfg)
	right := testDetection(0, 192, 100, 256, 220)
	events := tracker.Process("cam", "clip.mkv", 2, now.Add(time.Second), []Detection{right}, 320, 320, cfg)
	found := false
	for _, event := range events {
		if event.Type == "line_cross" && event.LineID == "l1" && event.Direction != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("line crossing missing: %#v", events)
	}
}

func TestRuleMatchesClassZoneSchedule(t *testing.T) {
	rule := AlertRule{Enabled: true, ClassName: "person", EventType: "zone_enter", ZoneID: "z1", Start: "08:00", End: "18:00", Timezone: "UTC", Days: []int{1}}
	event := SmartEvent{ClassName: "person", Type: "zone_enter", ZoneID: "z1"}
	monday := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	if !ruleMatches(rule, event, monday) {
		t.Fatal("expected rule to match")
	}
	if ruleMatches(rule, event, monday.Add(10*time.Hour)) {
		t.Fatal("rule should not match outside schedule")
	}
}
