package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"onvif-visor/internal/media"
)

func TestParseRecordingPageQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/recordings?page=2&page_size=10&from=2026-09-22T00:00:00Z&to=2026-09-23T00:00:00Z", nil)
	page, pageSize, from, to, err := parseRecordingPageQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	if page != 2 || pageSize != 10 {
		t.Fatalf("page=%d pageSize=%d, want 2 and 10", page, pageSize)
	}
	if got := to.Sub(from); got != 24*time.Hour {
		t.Fatalf("range=%s, want 24h", got)
	}

	for _, rawURL := range []string{
		"/api/recordings?page=0",
		"/api/recordings?page_size=101",
		"/api/recordings?from=2026-09-22T00:00:00Z",
		"/api/recordings?from=2026-09-22T00:00:00Z&to=2026-09-25T00:00:00Z",
	} {
		req := httptest.NewRequest("GET", rawURL, nil)
		if _, _, _, _, err := parseRecordingPageQuery(req); err == nil {
			t.Fatalf("parseRecordingPageQuery(%q) accepted invalid query", rawURL)
		}
	}
}

func TestRecordingsInRangeIncludesOverlap(t *testing.T) {
	day := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	files := []media.RecordingFile{
		{CameraID: "cam", Name: "previous.mkv", StartedAt: day.Add(-30 * time.Second), DurationSecond: 60},
		{CameraID: "cam", Name: "inside.mkv", StartedAt: day.Add(2 * time.Hour), DurationSecond: 30},
		{CameraID: "cam", Name: "next.mkv", StartedAt: day.Add(24 * time.Hour), DurationSecond: 30},
	}
	got := recordingsInRange(files, day, day.Add(24*time.Hour))
	if len(got) != 2 || got[0].Name != "previous.mkv" || got[1].Name != "inside.mkv" {
		t.Fatalf("recordingsInRange() = %#v, want previous+inside", got)
	}
}

func TestPaginateRecordingsClampsPage(t *testing.T) {
	files := make([]media.RecordingFile, 55)
	for i := range files {
		files[i].Name = time.Date(2026, 9, 22, 12, 0, i, 0, time.UTC).Format("150405") + ".mkv"
	}
	items, page, totalPages := paginateRecordings(files, 99, 24)
	if page != 3 || totalPages != 3 || len(items) != 7 {
		t.Fatalf("page=%d totalPages=%d items=%d, want 3, 3, 7", page, totalPages, len(items))
	}
}

func TestSummarizeRecordings(t *testing.T) {
	start := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	summary := summarizeRecordings("cam", []media.RecordingFile{
		{CameraID: "cam", StartedAt: start, SizeBytes: 100, DurationSecond: 10},
		{CameraID: "cam", StartedAt: start.Add(time.Hour), SizeBytes: 250, DurationSecond: 20},
	})
	if summary.Count != 2 || summary.SizeBytes != 350 || summary.DurationSecond != 30 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.LatestStarted == nil || !summary.LatestStarted.Equal(start.Add(time.Hour)) {
		t.Fatalf("latest=%v, want %v", summary.LatestStarted, start.Add(time.Hour))
	}
}
