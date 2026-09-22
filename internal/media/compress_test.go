package media

import (
	"testing"
	"time"
)

func TestCompressionSuitableForClip(t *testing.T) {
	started := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	path := "20260922-120000.000.mkv"
	if !compressionSuitableForClip(path, started.Add(10*time.Minute)) {
		t.Fatal("short event clip should remain eligible for compression")
	}
	if compressionSuitableForClip(path, started.Add(7*time.Hour)) {
		t.Fatal("long continuous recording should not be transcoded after closing")
	}
	if !compressionSuitableForClip("legacy-name.mkv", started.Add(24*time.Hour)) {
		t.Fatal("unknown legacy names should keep previous compression behavior")
	}
}

func TestShouldReplaceWithCompressed(t *testing.T) {
	tests := []struct {
		name       string
		original   int64
		compressed int64
		want       bool
	}{
		{name: "smaller enough", original: 1000, compressed: 900, want: true},
		{name: "barely smaller", original: 1000, compressed: 980, want: false},
		{name: "larger", original: 1000, compressed: 1100, want: false},
		{name: "empty", original: 1000, compressed: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReplaceWithCompressed(tt.original, tt.compressed); got != tt.want {
				t.Fatalf("shouldReplaceWithCompressed(%d, %d) = %v, want %v", tt.original, tt.compressed, got, tt.want)
			}
		})
	}
}
