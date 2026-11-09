package media

import (
	"bufio"
	"bytes"
	"context"
	"testing"
	"time"
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

func TestLiveHubPublishReachesSubscribers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &liveHub{subs: make(map[chan []byte]struct{}), ctx: ctx, cancel: cancel}
	ch := h.subscribe()
	h.publish([]byte("frame-1"))
	select {
	case got := <-ch:
		if string(got) != "frame-1" {
			t.Fatalf("frame = %q, want frame-1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive the published frame")
	}
	// Un suscriptor lento conserva solo el fotograma más reciente.
	h.publish([]byte("frame-2"))
	h.publish([]byte("frame-3"))
	select {
	case got := <-ch:
		if string(got) != "frame-3" {
			t.Fatalf("frame = %q, want newest frame-3", got)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive the newest frame")
	}
}

func TestLiveHubStopOnlyWithoutSubscribers(t *testing.T) {
	m := &Manager{hubs: make(map[string]*liveHub)}
	ctx, cancel := context.WithCancel(context.Background())
	h := &liveHub{key: "cam|full", subs: make(map[chan []byte]struct{}), ctx: ctx, cancel: cancel}
	m.hubs[h.key] = h
	ch := h.subscribe()
	m.stopHub(h)
	if ctx.Err() != nil {
		t.Fatal("hub with subscribers must not stop")
	}
	m.unsubscribe(h, ch)
	// La baja del último suscriptor programa la parada con gracia; no es inmediata.
	if ctx.Err() != nil {
		t.Fatal("hub must keep its grace period after the last unsubscribe")
	}
	m.stopHub(h)
	if ctx.Err() == nil {
		t.Fatal("hub without subscribers must stop")
	}
	if _, ok := m.hubs[h.key]; ok {
		t.Fatal("stopped hub must be removed from the manager")
	}
}

func TestLiveFFmpegArgsSinOpcionesInexistentes(t *testing.T) {
	// Regresión: -rw_timeout no existe en el FFmpeg 9 del host y el directo
	// se quedaba sin fotogramas. Ningún argumento puede volver a colarse sin
	// probarse contra ese FFmpeg.
	args := liveFFmpegArgs("rtsp://camara/stream", "full")
	for _, arg := range args {
		if arg == "-rw_timeout" {
			t.Fatal("live args must not use -rw_timeout: not supported by host ffmpeg")
		}
	}
	joined := ""
	for _, arg := range args {
		joined += arg + " "
	}
	for _, want := range []string{"-rtsp_transport tcp", "nobuffer", "image2pipe", "mjpeg"} {
		if !bytes.Contains([]byte(joined), []byte(want)) {
			t.Fatalf("live args missing %q: %s", want, joined)
		}
	}
}
