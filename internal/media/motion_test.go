package media

import "testing"

func TestFrameDifferenceScoreIgnoresUniformBrightnessShift(t *testing.T) {
	previous := make([]byte, 100)
	current := make([]byte, 100)
	for i := range previous {
		previous[i] = 80
		current[i] = 100
	}
	if score := frameDifferenceScore(previous, current); score != 0 {
		t.Fatalf("score = %v, want 0 for uniform brightness change", score)
	}
}

func TestFrameDifferenceScoreDetectsLocalMotion(t *testing.T) {
	previous := make([]byte, 100)
	current := make([]byte, 100)
	for i := range previous {
		previous[i] = 80
		current[i] = 80
	}
	for i := 0; i < 20; i++ {
		current[i] = 140
	}
	if score := frameDifferenceScore(previous, current); score < 0.15 {
		t.Fatalf("score = %v, want clear local motion", score)
	}
}

func TestMotionStreamURIUsesSubstreamWhenAvailable(t *testing.T) {
	input := "rtsp://user:pass@192.0.2.5/cam/realmonitor?channel=1&subtype=0&unicast=true"
	got := motionStreamURI(input)
	if got == input || got == "" {
		t.Fatalf("motionStreamURI() = %q", got)
	}
	if want := "subtype=1"; !contains(got, want) {
		t.Fatalf("motionStreamURI() = %q, want %q", got, want)
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
