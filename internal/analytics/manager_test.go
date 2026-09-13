package analytics

import (
	"context"
	"testing"
	"time"
)

func TestWaitForPersonFrameHonorsNotBefore(t *testing.T) {
	m := &Manager{personFrames: map[string]PersonFrame{}}
	now := time.Now().UTC()
	m.personFrames["cam"] = PersonFrame{DetectedAt: now.Add(-5 * time.Second), Confidence: .5}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(30 * time.Millisecond)
		m.mu.Lock()
		m.personFrames["cam"] = PersonFrame{DetectedAt: now.Add(time.Second), Confidence: .9}
		m.mu.Unlock()
	}()
	frame, ok := m.WaitForPersonFrame(ctx, "cam", now)
	if !ok {
		t.Fatal("expected a recent person frame")
	}
	if frame.Confidence != .9 || !frame.DetectedAt.After(now) {
		t.Fatalf("unexpected frame: %#v", frame)
	}
}

func TestFitWithinPreservesAspectRatio(t *testing.T) {
	width, height := fitWithin(2304, 1296, InputWidth, InputHeight)
	if width != 320 || height != 180 {
		t.Fatalf("fitWithin(16:9) = %dx%d, want 320x180", width, height)
	}
	width, height = fitWithin(640, 480, InputWidth, InputHeight)
	if width != 320 || height != 240 {
		t.Fatalf("fitWithin(4:3) = %dx%d, want 320x240", width, height)
	}
}

func TestLetterboxBGRKeepsDisplayFrameAspect(t *testing.T) {
	width, height := 320, 180
	frame := make([]byte, width*height*3)
	for i := range frame {
		frame[i] = 77
	}
	boxed := letterboxBGR(frame, width, height, InputWidth, InputHeight)
	if len(boxed) != InputWidth*InputHeight*3 {
		t.Fatalf("letterbox size=%d", len(boxed))
	}
	// 320x180 centrado en 320x320 deja 70 filas negras arriba y abajo.
	if boxed[0] != 0 {
		t.Fatalf("top padding is not black: %d", boxed[0])
	}
	contentStart := 70 * InputWidth * 3
	if boxed[contentStart] != 77 {
		t.Fatalf("content did not start at expected row: %d", boxed[contentStart])
	}
}
