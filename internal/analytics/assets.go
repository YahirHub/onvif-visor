package analytics

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// Official NanoDet-Plus-m 320 ONNX model from RangiLyu/nanodet v1.0.0-alpha-1.
//
//go:embed assets/nanodet-plus-m_320.onnx
var embeddedModel []byte

//go:embed assets/NANODET_LICENSE.txt
var embeddedLicense []byte

func EnsureModel(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create analytics model dir: %w", err)
	}
	path := filepath.Join(dir, ModelFileName)
	want := sha256.Sum256(embeddedModel)
	modelOK := false
	if current, err := os.ReadFile(path); err == nil {
		modelOK = sha256.Sum256(current) == want
	}
	if !modelOK {
		if err := writeAssetAtomic(path, embeddedModel, 0o644); err != nil {
			return "", fmt.Errorf("write NanoDet model: %w", err)
		}
	}
	licensePath := filepath.Join(dir, "NANODET_LICENSE.txt")
	if current, err := os.ReadFile(licensePath); err != nil || sha256.Sum256(current) != sha256.Sum256(embeddedLicense) {
		if err := writeAssetAtomic(licensePath, embeddedLicense, 0o644); err != nil {
			return "", fmt.Errorf("write NanoDet license: %w", err)
		}
	}
	return path, nil
}

func EmbeddedModelSHA256() string { return fmt.Sprintf("%x", sha256.Sum256(embeddedModel)) }

func writeAssetAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".analytics-asset-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
