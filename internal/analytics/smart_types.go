package analytics

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const AnalysisModelKey = ModelName + ":objects-v2"

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Zone struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}

type Line struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	A    Point  `json:"a"`
	B    Point  `json:"b"`
}

type AlertRule struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	ClassName       string `json:"class_name"`
	EventType       string `json:"event_type"`
	ZoneID          string `json:"zone_id,omitempty"`
	LineID          string `json:"line_id,omitempty"`
	Start           string `json:"start,omitempty"`
	End             string `json:"end,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	Days            []int  `json:"days,omitempty"`
	Telegram        bool   `json:"telegram"`
	CooldownSeconds int    `json:"cooldown_seconds,omitempty"`
}

type CameraAnalyticsConfig struct {
	EnabledClasses []string    `json:"enabled_classes"`
	Zones          []Zone      `json:"zones"`
	Lines          []Line      `json:"lines"`
	Rules          []AlertRule `json:"rules"`
}

type ObjectSegment struct {
	ID            int64   `json:"id"`
	CameraID      string  `json:"camera_id"`
	RecordingName string  `json:"recording_name"`
	ClassID       int     `json:"class_id"`
	ClassName     string  `json:"class_name"`
	StartSecond   float64 `json:"start_seconds"`
	EndSecond     float64 `json:"end_seconds"`
	PeakSecond    float64 `json:"peak_seconds"`
	MaxConfidence float64 `json:"max_confidence"`
	BestScore     float64 `json:"best_score"`
	Source        string  `json:"source"`
}

type SmartEvent struct {
	ID               int64     `json:"id,omitempty"`
	CameraID         string    `json:"camera_id"`
	RecordingName    string    `json:"recording_name,omitempty"`
	OffsetSecond     float64   `json:"offset_seconds,omitempty"`
	OccurredAt       time.Time `json:"occurred_at"`
	ClassID          int       `json:"class_id"`
	ClassName        string    `json:"class_name"`
	TrackID          int64     `json:"track_id"`
	Type             string    `json:"type"`
	ZoneID           string    `json:"zone_id,omitempty"`
	ZoneName         string    `json:"zone_name,omitempty"`
	LineID           string    `json:"line_id,omitempty"`
	LineName         string    `json:"line_name,omitempty"`
	Direction        string    `json:"direction,omitempty"`
	Confidence       float64   `json:"confidence"`
	BestScore        float64   `json:"best_score,omitempty"`
	MatchedRuleIDs   []string  `json:"matched_rule_ids,omitempty"`
	MatchedRuleNames []string  `json:"matched_rule_names,omitempty"`
	NotifyTelegram   bool      `json:"notify_telegram,omitempty"`
}

type ClassInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Group string `json:"group"`
}

var defaultEnabledClasses = []string{"person", "bicycle", "car", "motorcycle", "bus", "truck", "cat", "dog"}

var classLabelsES = map[string]string{
	"person": "Persona", "bicycle": "Bicicleta", "car": "Automóvil", "motorcycle": "Motocicleta", "airplane": "Avión",
	"bus": "Autobús", "train": "Tren", "truck": "Camión", "boat": "Barco", "traffic light": "Semáforo", "fire hydrant": "Hidrante",
	"stop sign": "Señal de alto", "bench": "Banca", "bird": "Ave", "cat": "Gato", "dog": "Perro", "horse": "Caballo",
	"backpack": "Mochila", "umbrella": "Paraguas", "handbag": "Bolso", "suitcase": "Maleta", "sports ball": "Balón",
	"bottle": "Botella", "cup": "Taza", "chair": "Silla", "couch": "Sofá", "potted plant": "Planta", "dining table": "Mesa",
	"tv": "Televisión", "laptop": "Laptop", "cell phone": "Teléfono", "book": "Libro", "clock": "Reloj",
}

func DefaultCameraAnalyticsConfig() CameraAnalyticsConfig {
	return CameraAnalyticsConfig{EnabledClasses: append([]string(nil), defaultEnabledClasses...)}
}

func Classes() []ClassInfo {
	out := make([]ClassInfo, 0, len(COCOClasses))
	for id, name := range COCOClasses {
		label := classLabelsES[name]
		if label == "" {
			label = strings.ToUpper(name[:1]) + name[1:]
		}
		out = append(out, ClassInfo{ID: id, Name: name, Label: label, Group: classGroup(id)})
	}
	return out
}

func classGroup(id int) string {
	switch {
	case id == 0:
		return "personas"
	case id >= 1 && id <= 8:
		return "vehículos"
	case id >= 14 && id <= 23:
		return "animales"
	default:
		return "objetos"
	}
}

func classIDByName(name string) (int, bool) {
	for id, candidate := range COCOClasses {
		if candidate == name {
			return id, true
		}
	}
	return 0, false
}

func normalizeAnalyticsConfig(cfg CameraAnalyticsConfig) (CameraAnalyticsConfig, error) {
	if len(cfg.EnabledClasses) == 0 {
		cfg.EnabledClasses = append([]string(nil), defaultEnabledClasses...)
	}
	seenClasses := make(map[string]bool)
	classes := make([]string, 0, len(cfg.EnabledClasses))
	for _, name := range cfg.EnabledClasses {
		name = strings.TrimSpace(name)
		if _, ok := classIDByName(name); !ok {
			return CameraAnalyticsConfig{}, fmt.Errorf("clase NanoDet inválida: %s", name)
		}
		if !seenClasses[name] {
			seenClasses[name] = true
			classes = append(classes, name)
		}
	}
	if len(classes) > 40 {
		return CameraAnalyticsConfig{}, errors.New("máximo 40 clases habilitadas por cámara")
	}
	cfg.EnabledClasses = classes

	if len(cfg.Zones) > 16 || len(cfg.Lines) > 16 || len(cfg.Rules) > 32 {
		return CameraAnalyticsConfig{}, errors.New("demasiadas zonas, líneas o reglas")
	}
	zoneIDs := make(map[string]bool)
	for i := range cfg.Zones {
		zone := &cfg.Zones[i]
		zone.Name = strings.TrimSpace(zone.Name)
		if zone.Name == "" || len(zone.Name) > 64 || len(zone.Points) < 3 || len(zone.Points) > 24 {
			return CameraAnalyticsConfig{}, errors.New("zona inválida")
		}
		if zone.ID == "" {
			zone.ID = randomAnalyticsID("z")
		}
		if zoneIDs[zone.ID] {
			return CameraAnalyticsConfig{}, errors.New("ID de zona duplicado")
		}
		zoneIDs[zone.ID] = true
		for _, p := range zone.Points {
			if !validPoint(p) {
				return CameraAnalyticsConfig{}, errors.New("coordenada de zona inválida")
			}
		}
	}
	lineIDs := make(map[string]bool)
	for i := range cfg.Lines {
		line := &cfg.Lines[i]
		line.Name = strings.TrimSpace(line.Name)
		if line.Name == "" || len(line.Name) > 64 || !validPoint(line.A) || !validPoint(line.B) || distancePoints(line.A, line.B) < 0.02 {
			return CameraAnalyticsConfig{}, errors.New("línea inválida")
		}
		if line.ID == "" {
			line.ID = randomAnalyticsID("l")
		}
		if lineIDs[line.ID] {
			return CameraAnalyticsConfig{}, errors.New("ID de línea duplicado")
		}
		lineIDs[line.ID] = true
	}
	ruleIDs := make(map[string]bool)
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" || len(rule.Name) > 80 {
			return CameraAnalyticsConfig{}, errors.New("regla inválida")
		}
		if rule.ID == "" {
			rule.ID = randomAnalyticsID("r")
		}
		if ruleIDs[rule.ID] {
			return CameraAnalyticsConfig{}, errors.New("ID de regla duplicado")
		}
		ruleIDs[rule.ID] = true
		if rule.ClassName != "*" {
			if _, ok := classIDByName(rule.ClassName); !ok {
				return CameraAnalyticsConfig{}, errors.New("clase de regla inválida")
			}
		}
		switch rule.EventType {
		case "object", "zone_enter", "zone_exit", "line_cross":
		default:
			return CameraAnalyticsConfig{}, errors.New("tipo de regla inválido")
		}
		if strings.HasPrefix(rule.EventType, "zone_") && !zoneIDs[rule.ZoneID] {
			return CameraAnalyticsConfig{}, errors.New("zona de regla inexistente")
		}
		if rule.EventType == "line_cross" && !lineIDs[rule.LineID] {
			return CameraAnalyticsConfig{}, errors.New("línea de regla inexistente")
		}
		if rule.Start != "" || rule.End != "" {
			if !validClock(rule.Start) || !validClock(rule.End) {
				return CameraAnalyticsConfig{}, errors.New("horario de regla inválido")
			}
		}
		rule.Timezone = strings.TrimSpace(rule.Timezone)
		if rule.Timezone == "" {
			rule.Timezone = "Local"
		}
		if rule.Timezone != "Local" {
			if _, err := time.LoadLocation(rule.Timezone); err != nil {
				return CameraAnalyticsConfig{}, errors.New("zona horaria de regla inválida")
			}
		}
		if rule.CooldownSeconds < 0 || rule.CooldownSeconds > 3600 {
			return CameraAnalyticsConfig{}, errors.New("cooldown de regla inválido")
		}
		if rule.CooldownSeconds == 0 {
			rule.CooldownSeconds = 30
		}
		seenDays := make(map[int]bool)
		cleanDays := make([]int, 0, len(rule.Days))
		for _, day := range rule.Days {
			if day < 0 || day > 6 {
				return CameraAnalyticsConfig{}, errors.New("día de regla inválido")
			}
			if !seenDays[day] {
				seenDays[day] = true
				cleanDays = append(cleanDays, day)
			}
		}
		sort.Ints(cleanDays)
		rule.Days = cleanDays
	}
	return cfg, nil
}

func validPoint(p Point) bool { return p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1 }

func validClock(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	_, err := time.Parse("15:04", value)
	return err == nil
}

func randomAnalyticsID(prefix string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%x", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

func distancePoints(a, b Point) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Hypot(dx, dy)
}
