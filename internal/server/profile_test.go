package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileUpdateRequiresCurrentPasswordAndKeepsSession(t *testing.T) {
	accounts := testAccountStore(t)
	user, err := accounts.Authenticate(context.Background(), "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	token, session, err := accounts.CreateSession(context.Background(), user, "192.0.2.1", "browser")
	if err != nil {
		t.Fatal(err)
	}
	app := &App{accounts: accounts, authEnabled: true}

	handler := app.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.handleProfileUpdate(w, r)
	}))
	request := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(`{"username":"operador","current_password":"demo","new_password":"nueva-clave-segura"}`))
	request.AddCookie(&http.Cookie{Name: uiSessionCookie, Value: token})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("profile update status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"username":"operador"`) || !strings.Contains(recorder.Body.String(), `"password_changed":true`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
	validated, err := accounts.ValidateSession(context.Background(), token)
	if err != nil || validated.ID != session.ID || validated.Username != "operador" {
		t.Fatalf("current session lost after profile update: %#v err=%v", validated, err)
	}
}

func TestProfileUsernameValidation(t *testing.T) {
	valid := []string{"demo", "usuario_1", "José-2", "a.b"}
	for _, value := range valid {
		if err := validateProfileUsername(value); err != nil {
			t.Fatalf("validateProfileUsername(%q) = %v", value, err)
		}
	}
	invalid := []string{"ab", "usuario con espacio", "user@host"}
	for _, value := range invalid {
		if err := validateProfileUsername(value); err == nil {
			t.Fatalf("validateProfileUsername(%q) expected error", value)
		}
	}
}
