package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"onvif-visor/internal/account"
)

func testAccountStore(t *testing.T) *account.Store {
	t.Helper()
	s, err := account.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureBootstrapUser(context.Background(), "demo", "demo"); err != nil {
		s.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionLoginPersistenceAndProtectedRoute(t *testing.T) {
	accounts := testAccountStore(t)
	app := &App{accounts: accounts, authEnabled: true}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	statusRec := httptest.NewRecorder()
	app.handleSessionStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), `"authenticated":false`) {
		t.Fatalf("unexpected unauthenticated status: code=%d body=%s", statusRec.Code, statusRec.Body.String())
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":"demo","password":"bad"}`))
	badRec := httptest.NewRecorder()
	app.handleSessionLogin(badRec, badReq)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("bad credentials status = %d, want %d", badRec.Code, http.StatusUnauthorized)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"username":"demo","password":"demo"}`))
	loginReq.RemoteAddr = "192.0.2.10:54321"
	loginReq.Header.Set("User-Agent", "test-browser")
	loginRec := httptest.NewRecorder()
	app.handleSessionLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d body=%s", loginRec.Code, http.StatusNoContent, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value == "" {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}

	protected := app.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session, ok := currentSession(r); !ok || session.Username != "demo" {
			t.Fatalf("session context missing: %#v", session)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	unauthRec := httptest.NewRecorder()
	protected.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("protected route without session = %d, want %d", unauthRec.Code, http.StatusUnauthorized)
	}

	authReq := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	authReq.AddCookie(cookie)
	authRec := httptest.NewRecorder()
	protected.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusNoContent {
		t.Fatalf("protected route with session = %d, want %d", authRec.Code, http.StatusNoContent)
	}
}

func TestSessionListAndDeleteOthers(t *testing.T) {
	accounts := testAccountStore(t)
	user, err := accounts.Authenticate(context.Background(), "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	token1, current, err := accounts.CreateSession(context.Background(), user, "192.0.2.1", "one")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := accounts.CreateSession(context.Background(), user, "192.0.2.2", "two"); err != nil {
		t.Fatal(err)
	}
	app := &App{accounts: accounts, authEnabled: true}
	handler := app.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.handleSessions(w, r)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: uiSessionCookie, Value: token1})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), current.ID) || !strings.Contains(rec.Body.String(), `"current":true`) {
		t.Fatalf("sessions response: code=%d body=%s", rec.Code, rec.Body.String())
	}

	deleteHandler := app.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.handleDeleteOtherSessions(w, r)
	}))
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sessions/others", nil)
	deleteReq.AddCookie(&http.Cookie{Name: uiSessionCookie, Value: token1})
	deleteRec := httptest.NewRecorder()
	deleteHandler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK || !strings.Contains(deleteRec.Body.String(), `"deleted":1`) {
		t.Fatalf("delete others: code=%d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestStaticAssetsRemainAvailableForLogin(t *testing.T) {
	accounts := testAccountStore(t)
	app := &App{accounts: accounts, authEnabled: true}
	handler := app.auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("static route status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSessionLogoutClearsCookieAndDatabaseSession(t *testing.T) {
	accounts := testAccountStore(t)
	user, err := accounts.Authenticate(context.Background(), "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := accounts.CreateSession(context.Background(), user, "", "")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{accounts: accounts, authEnabled: true}
	req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	req.AddCookie(&http.Cookie{Name: uiSessionCookie, Value: token})
	rec := httptest.NewRecorder()
	app.handleSessionLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout cookie not expired: %#v", cookies)
	}
	if _, err := accounts.ValidateSession(context.Background(), token); err != account.ErrSessionNotFound {
		t.Fatalf("session still valid after logout: %v", err)
	}
}

func TestUIRoutesServeApplicationShell(t *testing.T) {
	app := &App{
		assets: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>ONVIF Visor</title><div id=\"appView\"></div>")},
			"styles.css": &fstest.MapFile{Data: []byte("body{}")},
		},
		mux: http.NewServeMux(),
	}
	app.routes()

	for _, path := range []string{"/", "/camaras", "/camara/cam-1", "/multivista", "/grabaciones", "/grabaciones/cam-1", "/eventos", "/sesiones", "/integraciones"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		app.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusOK)
			continue
		}
		if !strings.Contains(rec.Body.String(), `id="appView"`) {
			t.Errorf("GET %s did not serve application shell: %s", path, rec.Body.String())
		}
	}
}
