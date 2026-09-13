package analytics

import "math"

// scoreDetectionFrame favorece un frame útil para miniatura, no sólo el de
// mayor confianza. Todo se calcula sobre el tensor 320x320 ya decodificado y
// cuesta mucho menos que una segunda inferencia.
func scoreDetectionFrame(frameBGR []byte, detection Detection) float64 {
	box := detection.Box
	width := float64(max32(0, box.X2-box.X1))
	height := float64(max32(0, box.Y2-box.Y1))
	areaRatio := (width * height) / float64(InputWidth*InputHeight)
	sizeScore := math.Min(1, math.Sqrt(math.Max(0, areaRatio)/0.25))

	centerX := (float64(box.X1) + float64(box.X2)) / 2 / float64(InputWidth)
	centerY := (float64(box.Y1) + float64(box.Y2)) / 2 / float64(InputHeight)
	distance := math.Hypot(centerX-0.5, centerY-0.5) / math.Hypot(0.5, 0.5)
	centerScore := math.Max(0, 1-distance)

	sharpness := boxSharpness(frameBGR, box)
	confidence := math.Max(0, math.Min(1, float64(detection.Score)))
	return confidence*0.55 + sizeScore*0.20 + centerScore*0.15 + sharpness*0.10
}

func boxSharpness(frameBGR []byte, box Box) float64 {
	if len(frameBGR) != InputWidth*InputHeight*3 {
		return 0
	}
	x1 := clampInt(int(box.X1), 0, InputWidth-2)
	y1 := clampInt(int(box.Y1), 0, InputHeight-2)
	x2 := clampInt(int(box.X2), x1+1, InputWidth-1)
	y2 := clampInt(int(box.Y2), y1+1, InputHeight-1)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	const step = 4
	var total float64
	var samples int
	for y := y1; y < y2; y += step {
		for x := x1; x < x2; x += step {
			base := pixelLuma(frameBGR, x, y)
			right := pixelLuma(frameBGR, minInt(x+step, x2), y)
			down := pixelLuma(frameBGR, x, minInt(y+step, y2))
			total += math.Abs(right-base) + math.Abs(down-base)
			samples += 2
		}
	}
	if samples == 0 {
		return 0
	}
	// Un gradiente medio de ~48 niveles ya se considera suficientemente nítido.
	return math.Min(1, (total/float64(samples))/48.0)
}

func pixelLuma(frame []byte, x, y int) float64 {
	i := (y*InputWidth + x) * 3
	b, g, r := float64(frame[i]), float64(frame[i+1]), float64(frame[i+2])
	return 0.114*b + 0.587*g + 0.299*r
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func summarizeClassDetections(frameBGR []byte, detections []Detection, classID int) (maxConfidence, bestScore float64, found bool) {
	for _, detection := range detections {
		if detection.ClassID != classID {
			continue
		}
		found = true
		if confidence := float64(detection.Score); confidence > maxConfidence {
			maxConfidence = confidence
		}
		score := detection.BestScore
		if score <= 0 {
			score = scoreDetectionFrame(frameBGR, detection)
		}
		if score > bestScore {
			bestScore = score
		}
	}
	return maxConfidence, bestScore, found
}
