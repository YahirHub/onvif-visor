package analytics

import "sort"

type ObjectDetectionPoint struct {
	Second     float64
	ClassID    int
	ClassName  string
	Confidence float64
	BestScore  float64
}

func scoreDetections(frameBGR []byte, detections []Detection, enabled map[string]bool) []Detection {
	out := make([]Detection, len(detections))
	copy(out, detections)
	for i := range out {
		if enabled[out[i].ClassName] {
			out[i].BestScore = scoreDetectionFrame(frameBGR, out[i])
		}
	}
	return out
}

func enabledClassSet(cfg CameraAnalyticsConfig) map[string]bool {
	set := make(map[string]bool, len(cfg.EnabledClasses))
	for _, name := range cfg.EnabledClasses {
		set[name] = true
	}
	return set
}

func pointsForFrame(second float64, detections []Detection, enabled map[string]bool) []ObjectDetectionPoint {
	type aggregate struct {
		id            int
		maxConfidence float64
		bestScore     float64
	}
	byClass := make(map[string]aggregate)
	for _, detection := range detections {
		if !enabled[detection.ClassName] {
			continue
		}
		item := byClass[detection.ClassName]
		item.id = detection.ClassID
		if confidence := float64(detection.Score); confidence > item.maxConfidence {
			item.maxConfidence = confidence
		}
		if detection.BestScore > item.bestScore {
			item.bestScore = detection.BestScore
		}
		byClass[detection.ClassName] = item
	}
	out := make([]ObjectDetectionPoint, 0, len(byClass))
	for className, item := range byClass {
		out = append(out, ObjectDetectionPoint{Second: second, ClassID: item.id, ClassName: className, Confidence: item.maxConfidence, BestScore: item.bestScore})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ClassID < out[j].ClassID })
	return out
}

func MergeObjectDetectionPoints(points []ObjectDetectionPoint, maxGap float64) []ObjectSegment {
	if maxGap <= 0 {
		maxGap = 3
	}
	byClass := make(map[string][]ObjectDetectionPoint)
	for _, point := range points {
		byClass[point.ClassName] = append(byClass[point.ClassName], point)
	}
	segments := make([]ObjectSegment, 0)
	for className, classPoints := range byClass {
		sort.Slice(classPoints, func(i, j int) bool { return classPoints[i].Second < classPoints[j].Second })
		if len(classPoints) == 0 {
			continue
		}
		first := classPoints[0]
		current := ObjectSegment{ClassID: first.ClassID, ClassName: className, StartSecond: first.Second, EndSecond: first.Second + 1, PeakSecond: first.Second, MaxConfidence: first.Confidence, BestScore: first.BestScore, Source: "offline"}
		for _, point := range classPoints[1:] {
			if point.Second-current.EndSecond <= maxGap {
				if end := point.Second + 1; end > current.EndSecond {
					current.EndSecond = end
				}
				if point.Confidence > current.MaxConfidence {
					current.MaxConfidence = point.Confidence
				}
				if point.BestScore > current.BestScore {
					current.BestScore = point.BestScore
					current.PeakSecond = point.Second
				}
				continue
			}
			segments = append(segments, current)
			current = ObjectSegment{ClassID: point.ClassID, ClassName: className, StartSecond: point.Second, EndSecond: point.Second + 1, PeakSecond: point.Second, MaxConfidence: point.Confidence, BestScore: point.BestScore, Source: "offline"}
		}
		segments = append(segments, current)
	}
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StartSecond == segments[j].StartSecond {
			return segments[i].ClassID < segments[j].ClassID
		}
		return segments[i].StartSecond < segments[j].StartSecond
	})
	return segments
}
