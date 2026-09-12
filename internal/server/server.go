package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
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

const (
	uiSessionCookie = "onvif_viewer_session"
	uiSessionMaxAge = 12 * 60 * 60
)

type Config struct {
	Store      *store.Store
	Media      *media.Manager
	Assets     fs.FS
	UIUser     string
	UIPassword string
}

type App struct {
	store        *store.Store
	media        *media.Manager
	assets       fs.FS
	uiUser       string
	uiPassword   string
	sessionToken string
	mux          *http.ServeMux

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
	if (cfg.UIUser == "") != (cfg.UIPassword == "") {
		return nil, errors.New("ui user and password must be configured together")
	}

	sessionToken := ""
	if cfg.UIUser != "" {
		var err error
		sessionToken, err = newSessionToken()
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		store: cfg.Store, media: cfg.Media, assets: cfg.Assets,
		uiUser: cfg.UIUser, uiPassword: cfg.UIPassword, sessionToken: sessionToken,
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
	a.mux.HandleFunc("GET /api/session", a.handleSessionStatus)
	a.mux.HandleFunc("POST /api/session", a.handleSessionLogin)
	a.mux.HandleFunc("DELETE /api/session", a.handleSessionLogout)
	a.mux.HandleFunc("POST /api/discover", a.handleDiscover)
	a.mux.HandleFunc("GET /api/cameras", a.handleListCameras)
	a.mux.HandleFunc("POST /api/cameras", a.handleAddCamera)
	a.mux.HandleFunc("DELETE /api/cameras/{id}", a.handleDeleteCamera)
	a.mux.HandleFunc("PUT /api/cameras/{id}/recording", a.handleRecordingSettings)
	a.mux.HandleFunc("GET /api/cameras/{id}/live", a.handleLive)
	a.mux.HandleFunc("GET /api/cameras/{id}/snapshot", a.handleSnapshot)
	a.mux.HandleFunc("GET /api/events", a.handleEvents)
	a.mux.HandleFunc("GET /api/recordings", a.handleRecordings)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/snapshot", a.handleRecordingSnapshot)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/play", a.handleRecordingPlayback)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/download", a.handleRecordingDownload)
	a.mux.Handle("GET /", http.FileServer(http.FS(a.assets)))
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ffmpeg": true})
}

func (a *App) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	required := a.authRequired()
	writeJSON(w, http.StatusOK, map[string]bool{
		"required":      required,
		"authenticated": !required || a.hasValidSession(r),
	})
}

func (a *App) handleSessionLogin(w http.ResponseWriter, r *http.Request) {
	if !a.authRequired() {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Completa usuario y contraseña.")
		return
	}
	if !constantEqual(strings.TrimSpace(req.Username), a.uiUser) || !constantEqual(req.Password, a.uiPassword) {
		writeError(w, http.StatusUnauthorized, "Usuario o contraseña incorrectos.")
		return
	}

	expires := time.Now().Add(uiSessionMaxAge * time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    a.sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   uiSessionMaxAge,
		Expires:  expires,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authRequired() || r.URL.Path == "/api/health" || r.URL.Path == "/api/session" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if !a.hasValidSession(r) {
			writeError(w, http.StatusUnauthorized, "La sesión no es válida o expiró.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) authRequired() bool {
	return a.uiUser != "" && a.uiPassword != ""
}

func (a *App) hasValidSession(r *http.Request) bool {
	if !a.authRequired() {
		return true
	}
	cookie, err := r.Cookie(uiSessionCookie)
	if err != nil {
		return false
	}
	return constantEqual(cookie.Value, a.sessionToken)
}

func newSessionToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", errors.New("generate UI session token: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
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
