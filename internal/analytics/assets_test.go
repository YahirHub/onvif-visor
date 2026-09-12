package analytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureModelRestoresLicenseWithoutRewritingModel(t *testing.T) {
	dir := t.TempDir()
	modelPath, err := EnsureModel(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "NANODET_LICENSE.txt")
	if err := os.Remove(licensePath); err != nil {
		t.Fatal(err)
	}
	modelPath2, err := EnsureModel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if modelPath2 != modelPath {
		t.Fatalf("model path changed: %q != %q", modelPath2, modelPath)
	}
	after, err := os.Stat(modelPath2)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged model was rewritten")
	}
	license, err := os.ReadFile(licensePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(license) == 0 {
		t.Fatal("restored NanoDet license is empty")
	}
}
