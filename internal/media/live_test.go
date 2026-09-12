package media

import (
	"bufio"
	"bytes"
	"testing"
)

func TestSplitJPEGFrames(t *testing.T) {
	frame1 := []byte{0xff, 0xd8, 1, 2, 3, 0xff, 0xd9}
	frame2 := []byte{0xff, 0xd8, 9, 8, 7, 0xff, 0xd9}
	stream := append(append([]byte{0, 1, 2}, frame1...), frame2...)
	scanner := bufio.NewScanner(bytes.NewReader(stream))
	scanner.Split(splitJPEGFrames)
	var got [][]byte
	for scanner.Scan() {
		got = append(got, append([]byte(nil), scanner.Bytes()...))
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("frames = %d, want 2", len(got))
	}
	if !bytes.Equal(got[0], frame1) || !bytes.Equal(got[1], frame2) {
		t.Fatalf("unexpected frames: %#v", got)
	}
}

func TestSplitJPEGFramesRejectsTruncatedJPEG(t *testing.T) {
	scanner := bufio.NewScanner(bytes.NewReader([]byte{0xff, 0xd8, 1, 2, 3}))
	scanner.Split(splitJPEGFrames)
	for scanner.Scan() {
		t.Fatal("truncated frame should not be emitted")
	}
	if scanner.Err() == nil {
		t.Fatal("expected scanner error for truncated frame")
	}
}

func TestPushLatestFrameDropsStaleFrame(t *testing.T) {
	frames := make(chan []byte, 1)
	pushLatestFrame(frames, []byte("old"))
	pushLatestFrame(frames, []byte("new"))
	if got := string(<-frames); got != "new" {
		t.Fatalf("frame = %q, want newest frame", got)
	}
}
