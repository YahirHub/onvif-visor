package media

import (
	"testing"
	"time"
)

func TestNormalizeTelegramClipDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  time.Duration
	}{
		{input: time.Second, want: 3 * time.Second},
		{input: 30 * time.Second, want: 30 * time.Second},
		{input: 90 * time.Second, want: 60 * time.Second},
	}
	for _, tt := range tests {
		if got := normalizeTelegramClipDuration(tt.input); got != tt.want {
			t.Fatalf("normalizeTelegramClipDuration(%s) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
