package account

import (
	"context"
	"errors"
	"testing"
)

func TestUpdateProfileChangesCredentialsAndKeepsCurrentSession(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	user, err := s.EnsureBootstrapUser(ctx, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	currentToken, current, err := s.CreateSession(ctx, user, "192.0.2.1", "current")
	if err != nil {
		t.Fatal(err)
	}
	otherToken, _, err := s.CreateSession(ctx, user, "192.0.2.2", "other")
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.UpdateProfile(ctx, user.ID, current.ID, "demo", "operador", "nueva-clave-segura")
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Username != "operador" || !result.PasswordChanged || result.OtherSessionsClosed != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := s.Authenticate(ctx, "demo", "demo"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old credentials remain valid: %v", err)
	}
	if _, err := s.Authenticate(ctx, "operador", "nueva-clave-segura"); err != nil {
		t.Fatalf("new credentials rejected: %v", err)
	}
	if session, err := s.ValidateSession(ctx, currentToken); err != nil || session.Username != "operador" {
		t.Fatalf("current session invalid: %#v err=%v", session, err)
	}
	if _, err := s.ValidateSession(ctx, otherToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("other session still valid: %v", err)
	}
}

func TestUpdateProfileRejectsWrongCurrentPassword(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	_, session, err := s.CreateSession(ctx, user, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateProfile(ctx, user.ID, session.ID, "incorrecta", "nuevo", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("UpdateProfile() error = %v, want invalid credentials", err)
	}
}

func TestUpdateProfileRejectsDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, err := s.EnsureBootstrapUser(ctx, "uno", "clave-uno")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureBootstrapUser(ctx, "dos", "clave-dos"); err != nil {
		t.Fatal(err)
	}
	_, session, err := s.CreateSession(ctx, first, "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateProfile(ctx, first.ID, session.ID, "clave-uno", "dos", "")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("UpdateProfile() error = %v, want username taken", err)
	}
}
