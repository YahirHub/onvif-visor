//go:build !cgo

package analytics

import "fmt"

type disabledDetector struct{}

func (disabledDetector) Detect([]byte) ([]Detection, error) {
	return nil, ErrInferenceUnavailable
}
func (disabledDetector) Close() error  { return nil }
func (disabledDetector) Model() string { return ModelName }
func NewDetector(runtimePath, modelPath string) (Detector, error) {
	return nil, fmt.Errorf("%w: binary built with CGO_ENABLED=0", ErrInferenceUnavailable)
}
