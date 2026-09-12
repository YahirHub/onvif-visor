package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"onvif-visor/internal/account"
	"onvif-visor/internal/media"
	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

const uiSessionCookie = "onvif_viewer_session"

type Config struct {
	Store    *store.Store
	Accounts *account.Store
	Media    *media.Manager
	Assets   fs.FS
}

type App struct {
	store       *store.Store
	accounts    *account.Store
	media       *media.Manager
	assets      fs.FS
	authEnabled bool
	mux         *http.ServeMux

	ctx    context.Context
	cancel context.CancelFunc

	watchMu  sync.Mutex
	watchers map[string]context.CancelFunc

	eventsMu sync.Mutex
	events   []CameraEvent

	telegramMu           sync.Mutex
	telegramCancel       context.CancelFunc
	telegramNotifyMu     sync.Mutex
	telegramLastSent     map[string]time.Time
	telegramNotifySem    chan struct{}
	telegramAuthMu       sync.Mutex
	telegramAuthFailures map[int64][]time.Time
}

type CameraEvent struct {
	CameraID        string            `json:"camera_id"`
	CameraName      string            `json:"camera_name"`
	Topic           string            `json:"topic"`
	Time            time.Time         `json:"time"`
	Source          map[string]string `json:"source,omitempty"`
	Data            map[string]string `json:"data,omitempty"`
	Triggered       bool              `json:"recording_trigger"`
	RecordingName   string            `json:"recording_name,omitempty"`
	RecordingOffset float64           `json:"recording_offset,omitempty"`
}

type requestSessionKey struct{}

func New(cfg Config) (*App, error) {
	if cfg.Store == nil || cfg.Accounts == nil || cfg.Media == nil || cfg.Assets == nil {
		return nil, errors.New("store, accounts, media and assets are required")
	}
	ctxCheck, cancelCheck := context.WithTimeout(context.Background(), 5*time.Second)
	hasUsers, err := cfg.Accounts.HasUsers(ctxCheck)
	cancelCheck()
	if err != nil {
		return nil, fmt.Errorf("check configured accounts: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		store: cfg.Store, accounts: cfg.Accounts, media: cfg.Media, assets: cfg.Assets,
		authEnabled: hasUsers,
		mux:         http.NewServeMux(), ctx: ctx, cancel: cancel,
		watchers:             make(map[string]context.CancelFunc),
		telegramLastSent:     make(map[string]time.Time),
		telegramNotifySem:    make(chan struct{}, 1),
		telegramAuthFailures: make(map[int64][]time.Time),
	}
	app.routes()
	app.startExistingWatchers()
	app.startScheduler()
	app.startTelegramIntegration()
	return app, nil
}

func (a *App) Handler() http.Handler {
	return a.securityHeaders(a.auth(a.mux))
}

func (a *App) Close() {
	a.cancel()
	a.stopTelegramIntegration()
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
	a.mux.HandleFunc("GET /api/profile", a.handleProfile)
	a.mux.HandleFunc("PUT /api/profile", a.handleProfileUpdate)
	a.mux.HandleFunc("GET /api/sessions", a.handleSessions)
	a.mux.HandleFunc("DELETE /api/sessions/others", a.handleDeleteOtherSessions)
	a.mux.HandleFunc("DELETE /api/sessions/{id}", a.handleDeleteSession)
	a.mux.HandleFunc("POST /api/discover", a.handleDiscover)
	a.mux.HandleFunc("GET /api/cameras", a.handleListCameras)
	a.mux.HandleFunc("POST /api/cameras", a.handleAddCamera)
	a.mux.HandleFunc("DELETE /api/cameras/{id}", a.handleDeleteCamera)
	a.mux.HandleFunc("PUT /api/cameras/{id}/recording", a.handleRecordingSettings)
	a.mux.HandleFunc("PUT /api/cameras/{id}/schedule", a.handleScheduleSettings)
	a.mux.HandleFunc("GET /api/cameras/{id}/live", a.handleLive)
	a.mux.HandleFunc("GET /api/cameras/{id}/snapshot", a.handleSnapshot)
	a.mux.HandleFunc("GET /api/events", a.handleEvents)
	a.mux.HandleFunc("GET /api/recordings", a.handleRecordings)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/snapshot", a.handleRecordingSnapshot)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/play", a.handleRecordingPlayback)
	a.mux.HandleFunc("GET /api/recordings/{cameraID}/{name}/download", a.handleRecordingDownload)
	a.mux.HandleFunc("GET /api/integrations/telegram", a.handleTelegramStatus)
	a.mux.HandleFunc("PUT /api/integrations/telegram", a.handleTelegramSave)
	a.mux.HandleFunc("DELETE /api/integrations/telegram", a.handleTelegramDelete)
	a.mux.HandleFunc("POST /api/integrations/telegram/code", a.handleTelegramCode)
	a.mux.HandleFunc("GET /{$}", a.handleApp)
	a.mux.HandleFunc("GET /camaras", a.handleApp)
	a.mux.HandleFunc("GET /camara/{id}", a.handleApp)
	a.mux.HandleFunc("GET /multivista", a.handleApp)
	a.mux.HandleFunc("GET /grabaciones", a.handleApp)
	a.mux.HandleFunc("GET /grabaciones/{cameraID}", a.handleApp)
	a.mux.HandleFunc("GET /eventos", a.handleApp)
	a.mux.HandleFunc("GET /sesiones", a.handleApp)
	a.mux.HandleFunc("GET /perfil", a.handleApp)
	a.mux.HandleFunc("GET /integraciones", a.handleApp)
	a.mux.Handle("GET /", http.FileServer(http.FS(a.assets)))
}

func (a *App) handleApp(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(a.assets, "index.html")
	if err != nil {
		http.Error(w, "interfaz no disponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ffmpeg": true})
}

func (a *App) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{"required": a.authEnabled, "authenticated": !a.authEnabled}
	if !a.authEnabled {
		writeJSON(w, http.StatusOK, result)
		return
	}
	session, err := a.sessionFromCookie(r)
	if err == nil {
		result["authenticated"] = true
		result["username"] = session.Username
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleSessionLogin(w http.ResponseWriter, r *http.Request) {
	if !a.authEnabled {
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := a.accounts.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, account.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "Usuario o contraseña incorrectos.")
			return
		}
		log.Printf("action=login error=%v", err)
		writeError(w, http.StatusInternalServerError, "No se pudo iniciar sesión.")
		return
	}
	token, session, err := a.accounts.CreateSession(ctx, user, requestClientIP(r), r.UserAgent())
	if err != nil {
		log.Printf("action=create-session user=%d error=%v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "No se pudo crear la sesión.")
		return
	}
	setSessionCookie(w, r, token, session.ExpiresAt)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(uiSessionCookie); err == nil && cookie.Value != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		if err := a.accounts.DeleteSessionByToken(ctx, cookie.Value); err != nil {
			log.Printf("action=logout error=%v", err)
		}
		cancel()
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authEnabled || r.URL.Path == "/api/health" || r.URL.Path == "/api/session" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		session, err := a.sessionFromCookie(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "La sesión no es válida o expiró.")
			return
		}
		ctx := context.WithValue(r.Context(), requestSessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) sessionFromCookie(r *http.Request) (account.Session, error) {
	cookie, err := r.Cookie(uiSessionCookie)
	if err != nil || cookie.Value == "" {
		return account.Session{}, account.ErrSessionNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	return a.accounts.ValidateSession(ctx, cookie.Value)
}

func currentSession(r *http.Request) (account.Session, bool) {
	session, ok := r.Context().Value(requestSessionKey{}).(account.Session)
	return session, ok
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     uiSessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(account.SessionDuration.Seconds()),
		Expires:  expires,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: uiSessionCookie, Value: "", Path: "/", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
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

func (a *App) addEvent(cam store.Camera, event onvifsvc.Event, status media.RecordingStatus, triggered bool) {
	item := CameraEvent{
		CameraID: cam.ID, CameraName: cam.Name, Topic: event.Topic, Time: event.Time,
		Source: event.Source, Data: event.Data, Triggered: triggered,
	}
	if status.Path != "" && !status.StartedAt.IsZero() {
		item.RecordingName = filepath.Base(status.Path)
		item.RecordingOffset = event.Time.Sub(status.StartedAt).Seconds()
		if item.RecordingOffset < 0 {
			item.RecordingOffset = 0
		}
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

func logCameraError(cameraID, action string, err error) {
	log.Printf("camera=%s action=%s error=%v", cameraID, action, err)
}
