package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func containsPair(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}

func TestMP4RemuxArgsHEVC(t *testing.T) {
	args := mp4RemuxArgs("/rec/cam.mkv", "hevc", "aac", 0, 0)
	if !containsPair(args, "-c:v", "copy") {
		t.Fatalf("video must be copied, not re-encoded: %v", args)
	}
	if !containsPair(args, "-tag:v", "hvc1") {
		t.Fatalf("hevc must be tagged hvc1: %v", args)
	}
	if !containsPair(args, "-c:a", "copy") {
		t.Fatalf("aac audio must be copied: %v", args)
	}
	if !containsPair(args, "-f", "mp4") {
		t.Fatalf("output must be mp4: %v", args)
	}
}

func TestMP4RemuxArgsH264ConAudioG711(t *testing.T) {
	args := mp4RemuxArgs("/rec/cam.mkv", "h264", "pcm_alaw", 0, 0)
	if containsPair(args, "-tag:v", "hvc1") {
		t.Fatalf("h264 must not be tagged hvc1: %v", args)
	}
	if !containsPair(args, "-c:v", "copy") {
		t.Fatalf("video must be copied: %v", args)
	}
	if !containsPair(args, "-c:a", "aac") {
		t.Fatalf("g711 audio must be transcoded to aac: %v", args)
	}
}

func TestDownloadMP4GeneraMP4Valido(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg no disponible")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe no disponible")
	}
	dir := t.TempDir()
	cameraDir := filepath.Join(dir, "cam1")
	if err := os.MkdirAll(cameraDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "20260101-120000.000.mkv"
	src := filepath.Join(cameraDir, name)
	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440",
		"-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-y", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generar mkv de prueba: %v %s", err, out)
	}
	m, err := New("ffmpeg", dir)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/download?format=mp4", nil)
	rec := httptest.NewRecorder()
	if err := m.DownloadMP4(rec, req, "cam1", name, 0, 0); err != nil {
		t.Fatalf("DownloadMP4: %v", err)
	}
	body := rec.Body.Bytes()
	if len(body) < 1024 {
		t.Fatalf("mp4 demasiado pequeño: %d bytes", len(body))
	}
	if !bytes.Contains(body[:min(64, len(body))], []byte("ftyp")) {
		t.Fatal("la salida no empieza con una caja ftyp de MP4")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, ".mp4") {
		t.Fatalf("Content-Disposition sin nombre .mp4: %q", cd)
	}
	out := filepath.Join(dir, "salida.mp4")
	if err := os.WriteFile(out, body, 0o644); err != nil {
		t.Fatal(err)
	}
	probe := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=format_name,duration", "-of", "default=noprint_wrappers=1", out)
	probeOut, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe de la salida: %v %s", err, probeOut)
	}
	if !strings.Contains(string(probeOut), "mp4") {
		t.Fatalf("formato no reconocido como mp4: %s", probeOut)
	}
}

func TestMP4RemuxArgsConTramo(t *testing.T) {
	args := mp4RemuxArgs("/rec/cam.mkv", "hevc", "aac", 12.5, 30)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-ss 12.500") || !strings.Contains(joined, "-t 30.000") {
		t.Fatalf("el tramo no aparece en los argumentos: %v", args)
	}
	if strings.Index(joined, "-ss") > strings.Index(joined, "-i /rec/cam.mkv") {
		t.Fatalf("-ss debe ir antes de -i: %v", args)
	}
	sin := mp4RemuxArgs("/rec/cam.mkv", "hevc", "aac", 0, 0)
	if strings.Contains(strings.Join(sin, " "), "-ss") || strings.Contains(strings.Join(sin, " "), " -t ") {
		t.Fatalf("sin tramo no debe haber -ss ni -t: %v", sin)
	}
}
