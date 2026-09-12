package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"onvif-visor/internal/media"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

type Config struct {
	Store   *store.Store
	Media   *media.Manager
	Assets  fs.FS
	UIToken string
}

type App struct {
	store   *store.Store
	media   *media.Manager
	assets  fs.FS
	uiToken string
	mux     *http.ServeMux

	ctx    context.Context
	cancel context.CancelFunc

	watchMu  sync.Mutex
	watchers map[string]context.CancelFunc

	eventsMu sync.Mutex
	events   []CameraEvent
}

type CameraEvent struct {
	CameraID   string            `json:"camera_id"`
	CameraName string            `json:"camera_name"`
	Topic      string            `json:"topic"`
	Time       time.Time         `json:"time"`
	Source     map[string]string `json:"source,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
	Triggered  bool              `json:"recording_trigger"`
}

func New(cfg Config) (*App, error) {
	if cfg.Store == nil || cfg.Media == nil || cfg.Assets == nil {
		return nil, errors.New("store, media and assets are required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		store: cfg.Store, media: cfg.Media, assets: cfg.Assets, uiToken: cfg.UIToken,
		mux: http.NewServeMux(), ctx: ctx, cancel: cancel,
		watchers: make(map[string]context.CancelFunc),
	}
	app.routes()
	app.startExistingWatchers()
	return app, nil
}

func (a *App) Handler() http.Handler {
	return a.securityHeaders(a.auth(a.mux))
}

func (a *App) Close() {
	a.cancel()
	a.watchMu.Lock()
	for _, cancel := range a.watchers {
		cancel()
	}
	a.watchers = make(map[string]context.CancelFunc)
	a.watchMu.Unlock()
	a.media.StopAll()
}

func (a *App) routes() {
	a.mux.HandleFunc("GET /api/health", a.handleHealth)
	a.mux.HandleFunc("POST /api/session", a.handleSession)
	a.mux.HandleFunc("POST /api/discover", a.handleDiscover)
	a.mux.HandleFunc("GET /api/cameras", a.handleListCameras)
	a.mux.HandleFunc("POST /api/cameras", a.handleAddCamera)
	a.mux.HandleFunc("DELETE /api/cameras/{id}", a.handleDeleteCamera)
	a.mux.HandleFunc("PUT /api/cameras/{id}/recording", a.handleRecordingSettings)
	a.mux.HandleFunc("GET /api/cameras/{id}/live", a.handleLive)
	a.mux.HandleFunc("GET /api/events", a.handleEvents)
	a.mux.HandleFunc("GET /api/recordings", a.handleRecordings)
	a.mux.Handle("GET /", http.FileServer(http.FS(a.assets)))
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ffmpeg": true})
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	if a.uiToken == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil || !constantEqual(req.Token, a.uiToken) {
		writeError(w, http.StatusUnauthorized, "token inválido")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "onvif_viewer_session", Value: a.uiToken, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 60 * 60,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.uiToken == "" || r.URL.Path == "/api/health" || r.URL.Path == "/api/session" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if provided == "" {
			if cookie, err := r.Cookie("onvif_viewer_session"); err == nil {
				provided = cookie.Value
			}
		}
		if !constantEqual(provided, a.uiToken) {
			writeError(w, http.StatusUnauthorized, "autenticación requerida")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (a *App) addEvent(cam store.Camera, event onvifsvc.Event) {
	item := CameraEvent{
		CameraID: cam.ID, CameraName: cam.Name, Topic: event.Topic, Time: event.Time,
		Source: event.Source, Data: event.Data, Triggered: event.Recording,
	}
	a.eventsMu.Lock()
	a.events = append([]CameraEvent{item}, a.events...)
	if len(a.events) > 200 {
		a.events = a.events[:200]
	}
	a.eventsMu.Unlock()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func constantEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func logCameraError(cameraID, action string, err error) {
	log.Printf("camera=%s action=%s error=%v", cameraID, action, err)
}
