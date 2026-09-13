package analytics

import (
	"bytes"
	"context"
	"image/jpeg"
	"testing"
	"time"
)

func TestEncodeBGRJPEG(t *testing.T) {
	frame := make([]byte, 2*2*3)
	for i := 0; i < len(frame); i += 3 {
		frame[i] = 10
		frame[i+1] = 20
		frame[i+2] = 240
	}
	encoded, err := encodeBGRJPEG(frame, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("encoded frame is not JPEG: %v", err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("JPEG bounds = %v", img.Bounds())
	}
}

func TestWaitForPersonFrameHonorsNotBefore(t *testing.T) {
	m := &Manager{personFrames: map[string]PersonFrame{}}
	now := time.Now().UTC()
	m.personFrames["cam"] = PersonFrame{JPEG: []byte{1}, DetectedAt: now.Add(-5 * time.Second), Confidence: .5}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(30 * time.Millisecond)
		m.mu.Lock()
		m.personFrames["cam"] = PersonFrame{JPEG: []byte{2, 3}, DetectedAt: now.Add(time.Second), Confidence: .9}
		m.mu.Unlock()
	}()
	frame, ok := m.WaitForPersonFrame(ctx, "cam", now)
	if !ok {
		t.Fatal("expected a recent person frame")
	}
	if frame.Confidence != .9 || len(frame.JPEG) != 2 {
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
	encoded, err := encodeBGRJPEG(frame, width, height)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 180 {
		t.Fatalf("telegram frame bounds=%v, want 320x180", img.Bounds())
	}
}
