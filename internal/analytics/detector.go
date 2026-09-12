package analytics

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ModelName       = "nanodet-plus-m_320"
	ModelFileName   = "nanodet-plus-m_320.onnx"
	InputWidth      = 320
	InputHeight     = 320
	PersonThreshold = float32(0.35)
	NMSThreshold    = float32(0.50)
)

var ErrInferenceUnavailable = errors.New("analytics inference unavailable")

type Detector interface {
	DetectPerson(frameBGR []byte) (float32, bool, error)
	Close() error
	Model() string
}

type Box struct {
	X1, Y1, X2, Y2 float32
	Score          float32
}

func ResolveRuntimePath(explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("ONVIF_ONNXRUNTIME")); value != "" {
		return value
	}
	candidates := []string{
		"/usr/lib/libonnxruntime.so.1",
		"/usr/local/lib/libonnxruntime.so.1",
		"/usr/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	for _, root := range []string{"/usr/lib", "/usr/local/lib"} {
		matches, _ := filepath.Glob(filepath.Join(root, "libonnxruntime.so.*"))
		sort.Strings(matches)
		if len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}
	return ""
}

func nms(boxes []Box, threshold float32) []Box {
	if len(boxes) < 2 {
		return boxes
	}
	ordered := append([]Box(nil), boxes...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Score > ordered[j].Score })
	kept := make([]Box, 0, len(ordered))
	for len(ordered) > 0 {
		current := ordered[0]
		kept = append(kept, current)
		next := ordered[:0]
		for _, candidate := range ordered[1:] {
			if boxIoU(current, candidate) < threshold {
				next = append(next, candidate)
			}
		}
		ordered = next
	}
	return kept
}

func boxIoU(a, b Box) float32 {
	x1 := max32(a.X1, b.X1)
	y1 := max32(a.Y1, b.Y1)
	x2 := min32(a.X2, b.X2)
	y2 := min32(a.Y2, b.Y2)
	w := max32(0, x2-x1+1)
	h := max32(0, y2-y1+1)
	inter := w * h
	areaA := max32(0, a.X2-a.X1+1) * max32(0, a.Y2-a.Y1+1)
	areaB := max32(0, b.X2-b.X1+1) * max32(0, b.Y2-b.Y1+1)
	denom := areaA + areaB - inter
	if denom <= 0 {
		return 0
	}
	return inter / denom
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
