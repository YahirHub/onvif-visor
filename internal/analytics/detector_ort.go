//go:build cgo

package analytics

import (
	"fmt"
	"math"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	nanoClasses  = 80
	nanoRegMax   = 7
	nanoChannels = nanoClasses + (nanoRegMax+1)*4
	nanoPoints   = 2125
)

var nanoStrides = []int{8, 16, 32, 64}

type nanoDetORT struct {
	mu       sync.Mutex
	session  *ort.AdvancedSession
	input    *ort.Tensor[float32]
	output   *ort.Tensor[float32]
	envOwned bool
}

func NewDetector(runtimePath, modelPath string) (Detector, error) {
	if runtimePath == "" {
		return nil, fmt.Errorf("%w: libonnxruntime not found", ErrInferenceUnavailable)
	}
	if info, err := os.Stat(modelPath); err != nil || info.IsDir() {
		if err == nil {
			err = os.ErrNotExist
		}
		return nil, fmt.Errorf("%w: NanoDet model unavailable: %v", ErrInferenceUnavailable, err)
	}
	ort.SetSharedLibraryPath(runtimePath)
	owned := false
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(ort.WithLogLevelFatal()); err != nil {
			return nil, fmt.Errorf("%w: initialize ONNX Runtime: %v", ErrInferenceUnavailable, err)
		}
		owned = true
	}
	cleanup := func() {
		if owned && ort.IsInitialized() {
			_ = ort.DestroyEnvironment()
		}
	}
	input, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, InputHeight, InputWidth))
	if err != nil {
		cleanup()
		return nil, err
	}
	output, err := ort.NewEmptyTensor[float32](ort.NewShape(1, nanoPoints, nanoChannels))
	if err != nil {
		_ = input.Destroy()
		cleanup()
		return nil, err
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		_ = input.Destroy()
		_ = output.Destroy()
		cleanup()
		return nil, err
	}
	defer options.Destroy()
	if err := options.SetIntraOpNumThreads(1); err != nil {
		_ = input.Destroy()
		_ = output.Destroy()
		cleanup()
		return nil, err
	}
	if err := options.SetInterOpNumThreads(1); err != nil {
		_ = input.Destroy()
		_ = output.Destroy()
		cleanup()
		return nil, err
	}
	if err := options.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll); err != nil {
		_ = input.Destroy()
		_ = output.Destroy()
		cleanup()
		return nil, err
	}
	session, err := ort.NewAdvancedSession(modelPath, []string{"data"}, []string{"output"}, []ort.Value{input}, []ort.Value{output}, options)
	if err != nil {
		_ = input.Destroy()
		_ = output.Destroy()
		cleanup()
		return nil, fmt.Errorf("create NanoDet ONNX session: %w", err)
	}
	return &nanoDetORT{session: session, input: input, output: output, envOwned: owned}, nil
}

func (d *nanoDetORT) Model() string { return ModelName }

func (d *nanoDetORT) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var first error
	if d.session != nil {
		if err := d.session.Destroy(); err != nil && first == nil {
			first = err
		}
		d.session = nil
	}
	if d.input != nil {
		if err := d.input.Destroy(); err != nil && first == nil {
			first = err
		}
		d.input = nil
	}
	if d.output != nil {
		if err := d.output.Destroy(); err != nil && first == nil {
			first = err
		}
		d.output = nil
	}
	if d.envOwned && ort.IsInitialized() {
		if err := ort.DestroyEnvironment(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (d *nanoDetORT) DetectPerson(frameBGR []byte) (float32, bool, error) {
	if len(frameBGR) != InputWidth*InputHeight*3 {
		return 0, false, fmt.Errorf("invalid NanoDet frame size: %d", len(frameBGR))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.session == nil {
		return 0, false, ErrInferenceUnavailable
	}
	prepareNanoDetInput(d.input.GetData(), frameBGR)
	if err := d.session.Run(); err != nil {
		return 0, false, err
	}
	boxes := decodePersonBoxes(d.output.GetData(), PersonThreshold)
	boxes = nms(boxes, NMSThreshold)
	if len(boxes) == 0 {
		return 0, false, nil
	}
	return boxes[0].Score, true, nil
}

func prepareNanoDetInput(dst []float32, bgr []byte) {
	means := [3]float32{103.53, 116.28, 123.675}
	invStd := [3]float32{1 / 57.375, 1 / 57.12, 1 / 58.395}
	plane := InputWidth * InputHeight
	for i := 0; i < plane; i++ {
		base := i * 3
		for c := 0; c < 3; c++ {
			dst[c*plane+i] = (float32(bgr[base+c]) - means[c]) * invStd[c]
		}
	}
}

func decodePersonBoxes(pred []float32, threshold float32) []Box {
	if len(pred) < nanoPoints*nanoChannels {
		return nil
	}
	boxes := make([]Box, 0, 16)
	index := 0
	for _, stride := range nanoStrides {
		featW := int(math.Ceil(float64(InputWidth) / float64(stride)))
		featH := int(math.Ceil(float64(InputHeight) / float64(stride)))
		for y := 0; y < featH; y++ {
			for x := 0; x < featW; x++ {
				row := pred[index*nanoChannels : (index+1)*nanoChannels]
				score := row[0]
				if score >= threshold {
					dis := [4]float32{}
					for side := 0; side < 4; side++ {
						start := nanoClasses + side*(nanoRegMax+1)
						dis[side] = softmaxExpectation(row[start:start+nanoRegMax+1]) * float32(stride)
					}
					cx, cy := float32(x*stride), float32(y*stride)
					boxes = append(boxes, Box{
						X1: max32(0, cx-dis[0]), Y1: max32(0, cy-dis[1]),
						X2: min32(InputWidth, cx+dis[2]), Y2: min32(InputHeight, cy+dis[3]), Score: score,
					})
				}
				index++
			}
		}
	}
	return boxes
}

func softmaxExpectation(values []float32) float32 {
	if len(values) == 0 {
		return 0
	}
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	var denominator, weighted float64
	for i, value := range values {
		exp := math.Exp(float64(value - maxValue))
		denominator += exp
		weighted += float64(i) * exp
	}
	if denominator == 0 {
		return 0
	}
	return float32(weighted / denominator)
}
