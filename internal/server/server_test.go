package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionLoginAndProtectedRoute(t *testing.T) {
	app := &App{uiUser: "demo", uiPassword: "demo", sessionToken: "session-secret"}

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
	loginRec := httptest.NewRecorder()
	app.handleSessionLogin(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusNoContent)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Value != "session-secret" {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}

	protected := app.auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestStaticAssetsRemainAvailableForLogin(t *testing.T) {
	app := &App{uiUser: "demo", uiPassword: "demo", sessionToken: "session-secret"}
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

func TestSessionLogoutClearsCookie(t *testing.T) {
	app := &App{uiUser: "demo", uiPassword: "demo", sessionToken: "session-secret"}
	req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	rec := httptest.NewRecorder()
	app.handleSessionLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout cookie not expired: %#v", cookies)
	}
}
