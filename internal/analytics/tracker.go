package analytics

import (
	"math"
	"sort"
	"time"
)

const (
	trackMaxAge       = 4 * time.Second
	trackMaxCenterGap = 0.45
)

type normalizedBox struct {
	X1, Y1, X2, Y2 float64
}

type trackedDetection struct {
	Detection Detection
	Box       normalizedBox
	Center    Point
}

type objectTrack struct {
	ID         int64
	ClassID    int
	ClassName  string
	Box        normalizedBox
	Center     Point
	LastSeen   time.Time
	Zones      map[string]bool
	LineSides  map[string]float64
	Confidence float64
	BestScore  float64
}

type Tracker struct {
	nextID   int64
	tracks   map[int64]*objectTrack
	ruleLast map[string]time.Time
}

func NewTracker() *Tracker {
	return &Tracker{nextID: 1, tracks: make(map[int64]*objectTrack), ruleLast: make(map[string]time.Time)}
}

func normalizeDetections(detections []Detection, sourceWidth, sourceHeight int) []trackedDetection {
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return nil
	}
	offsetX := float64(InputWidth-sourceWidth) / 2
	offsetY := float64(InputHeight-sourceHeight) / 2
	out := make([]trackedDetection, 0, len(detections))
	for _, detection := range detections {
		box := normalizedBox{
			X1: clamp01((float64(detection.Box.X1) - offsetX) / float64(sourceWidth)),
			Y1: clamp01((float64(detection.Box.Y1) - offsetY) / float64(sourceHeight)),
			X2: clamp01((float64(detection.Box.X2) - offsetX) / float64(sourceWidth)),
			Y2: clamp01((float64(detection.Box.Y2) - offsetY) / float64(sourceHeight)),
		}
		if box.X2 <= box.X1 || box.Y2 <= box.Y1 {
			continue
		}
		out = append(out, trackedDetection{
			Detection: detection,
			Box:       box,
			Center:    Point{X: (box.X1 + box.X2) / 2, Y: (box.Y1 + box.Y2) / 2},
		})
	}
	return out
}

func (t *Tracker) Process(cameraID, recordingName string, offset float64, occurredAt time.Time, detections []Detection, sourceWidth, sourceHeight int, cfg CameraAnalyticsConfig) []SmartEvent {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	enabled := enabledClassSet(cfg)
	normalized := normalizeDetections(detections, sourceWidth, sourceHeight)
	filtered := normalized[:0]
	for _, detection := range normalized {
		if enabled[detection.Detection.ClassName] {
			filtered = append(filtered, detection)
		}
	}
	normalized = filtered

	for id, track := range t.tracks {
		if occurredAt.Sub(track.LastSeen) > trackMaxAge {
			delete(t.tracks, id)
		}
	}

	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Detection.Score > normalized[j].Detection.Score })
	usedTracks := make(map[int64]bool)
	events := make([]SmartEvent, 0)
	for _, detection := range normalized {
		track := t.match(detection, occurredAt, usedTracks)
		isNew := track == nil
		var previousCenter Point
		if isNew {
			track = &objectTrack{
				ID: t.nextID, ClassID: detection.Detection.ClassID, ClassName: detection.Detection.ClassName,
				Zones: make(map[string]bool), LineSides: make(map[string]float64),
			}
			t.nextID++
			t.tracks[track.ID] = track
		} else {
			previousCenter = track.Center
		}
		usedTracks[track.ID] = true

		track.Box = detection.Box
		track.Center = detection.Center
		track.LastSeen = occurredAt
		track.Confidence = float64(detection.Detection.Score)
		track.BestScore = detection.Detection.BestScore

		base := SmartEvent{
			CameraID: cameraID, RecordingName: recordingName, OffsetSecond: offset, OccurredAt: occurredAt,
			ClassID: track.ClassID, ClassName: track.ClassName, TrackID: track.ID,
			Confidence: track.Confidence, BestScore: track.BestScore,
		}
		if isNew {
			event := base
			event.Type = "object"
			t.applyRules(&event, cfg, occurredAt)
			events = append(events, event)
		}

		for _, zone := range cfg.Zones {
			inside := pointInPolygon(track.Center, zone.Points)
			wasInside, known := track.Zones[zone.ID]
			if !known {
				track.Zones[zone.ID] = inside
				if inside {
					event := base
					event.Type, event.ZoneID, event.ZoneName = "zone_enter", zone.ID, zone.Name
					t.applyRules(&event, cfg, occurredAt)
					events = append(events, event)
				}
				continue
			}
			if inside != wasInside {
				track.Zones[zone.ID] = inside
				event := base
				event.ZoneID, event.ZoneName = zone.ID, zone.Name
				if inside {
					event.Type = "zone_enter"
				} else {
					event.Type = "zone_exit"
				}
				t.applyRules(&event, cfg, occurredAt)
				events = append(events, event)
			}
		}

		for _, line := range cfg.Lines {
			side := lineSide(track.Center, line)
			previousSide, known := track.LineSides[line.ID]
			track.LineSides[line.ID] = side
			if !known || isNew || previousSide == 0 || side == 0 || previousSide*side >= 0 {
				continue
			}
			if !segmentsIntersect(previousCenter, track.Center, line.A, line.B) {
				continue
			}
			event := base
			event.Type, event.LineID, event.LineName = "line_cross", line.ID, line.Name
			if previousSide < 0 && side > 0 {
				event.Direction = "A→B"
			} else {
				event.Direction = "B→A"
			}
			t.applyRules(&event, cfg, occurredAt)
			events = append(events, event)
		}
	}
	return events
}

func (t *Tracker) match(detection trackedDetection, now time.Time, used map[int64]bool) *objectTrack {
	var best *objectTrack
	bestCost := math.MaxFloat64
	for _, track := range t.tracks {
		if used[track.ID] || track.ClassID != detection.Detection.ClassID || now.Sub(track.LastSeen) > trackMaxAge {
			continue
		}
		distance := distancePoints(track.Center, detection.Center)
		iou := normalizedIoU(track.Box, detection.Box)
		if distance > trackMaxCenterGap && iou < 0.08 {
			continue
		}
		cost := distance - iou*0.2
		if cost < bestCost {
			best, bestCost = track, cost
		}
	}
	return best
}

func (t *Tracker) applyRules(event *SmartEvent, cfg CameraAnalyticsConfig, when time.Time) {
	for _, rule := range cfg.Rules {
		if !rule.Enabled || !ruleMatches(rule, *event, when) {
			continue
		}
		lastKey := event.CameraID + "\x00" + rule.ID
		if last := t.ruleLast[lastKey]; !last.IsZero() && when.Sub(last) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}
		t.ruleLast[lastKey] = when
		event.MatchedRuleIDs = append(event.MatchedRuleIDs, rule.ID)
		event.MatchedRuleNames = append(event.MatchedRuleNames, rule.Name)
		if rule.Telegram {
			event.NotifyTelegram = true
		}
	}
}

func ruleMatches(rule AlertRule, event SmartEvent, when time.Time) bool {
	if rule.ClassName != "*" && rule.ClassName != event.ClassName {
		return false
	}
	if rule.EventType != event.Type {
		return false
	}
	if rule.ZoneID != "" && rule.ZoneID != event.ZoneID {
		return false
	}
	if rule.LineID != "" && rule.LineID != event.LineID {
		return false
	}
	location := time.Local
	if rule.Timezone != "" && rule.Timezone != "Local" {
		if loaded, err := time.LoadLocation(rule.Timezone); err == nil {
			location = loaded
		}
	}
	local := when.In(location)
	if len(rule.Days) > 0 {
		matchedDay := false
		for _, day := range rule.Days {
			if day == int(local.Weekday()) {
				matchedDay = true
				break
			}
		}
		if !matchedDay {
			return false
		}
	}
	if rule.Start != "" && rule.End != "" {
		minute := local.Hour()*60 + local.Minute()
		start, _ := clockMinutes(rule.Start)
		end, _ := clockMinutes(rule.End)
		if start <= end {
			if minute < start || minute > end {
				return false
			}
		} else if minute < start && minute > end {
			return false
		}
	}
	return true
}

func clockMinutes(value string) (int, bool) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func pointInPolygon(point Point, polygon []Point) bool {
	if len(polygon) < 3 {
		return false
	}
	inside := false
	j := len(polygon) - 1
	for i := 0; i < len(polygon); i++ {
		pi, pj := polygon[i], polygon[j]
		intersects := (pi.Y > point.Y) != (pj.Y > point.Y) &&
			point.X < (pj.X-pi.X)*(point.Y-pi.Y)/(pj.Y-pi.Y)+pi.X
		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}

func lineSide(point Point, line Line) float64 {
	return (line.B.X-line.A.X)*(point.Y-line.A.Y) - (line.B.Y-line.A.Y)*(point.X-line.A.X)
}

func segmentsIntersect(a, b, c, d Point) bool {
	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)
	return o1*o2 <= 0 && o3*o4 <= 0
}

func orientation(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func normalizedIoU(a, b normalizedBox) float64 {
	x1, y1 := math.Max(a.X1, b.X1), math.Max(a.Y1, b.Y1)
	x2, y2 := math.Min(a.X2, b.X2), math.Min(a.Y2, b.Y2)
	intersection := math.Max(0, x2-x1) * math.Max(0, y2-y1)
	areaA := math.Max(0, a.X2-a.X1) * math.Max(0, a.Y2-a.Y1)
	areaB := math.Max(0, b.X2-b.X1) * math.Max(0, b.Y2-b.Y1)
	denom := areaA + areaB - intersection
	if denom <= 0 {
		return 0
	}
	return intersection / denom
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
