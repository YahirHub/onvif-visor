package analytics

import "sort"

type DetectionPoint struct {
	Second     float64
	Confidence float64
}

func MergeDetectionPoints(points []DetectionPoint, maxGap float64) []PersonSegment {
	if len(points) == 0 {
		return []PersonSegment{}
	}
	if maxGap <= 0 {
		maxGap = 3
	}
	ordered := append([]DetectionPoint(nil), points...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Second < ordered[j].Second })
	segments := make([]PersonSegment, 0)
	current := PersonSegment{
		StartSecond:   ordered[0].Second,
		EndSecond:     ordered[0].Second + 1,
		PeakSecond:    ordered[0].Second,
		MaxConfidence: ordered[0].Confidence,
		Source:        "offline",
	}
	for _, point := range ordered[1:] {
		if point.Second-current.EndSecond <= maxGap {
			if nextEnd := point.Second + 1; nextEnd > current.EndSecond {
				current.EndSecond = nextEnd
			}
			if point.Confidence > current.MaxConfidence {
				current.MaxConfidence = point.Confidence
				current.PeakSecond = point.Second
			}
			continue
		}
		segments = append(segments, current)
		current = PersonSegment{StartSecond: point.Second, EndSecond: point.Second + 1, PeakSecond: point.Second, MaxConfidence: point.Confidence, Source: "offline"}
	}
	segments = append(segments, current)
	return segments
}
