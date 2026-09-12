package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentSessionAndRevocation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	user, err := first.EnsureBootstrapUser(ctx, "demo", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Authenticate(ctx, "demo", "bad"); err != ErrInvalidCredentials {
		t.Fatalf("bad auth = %v", err)
	}
	token1, session1, err := first.CreateSession(ctx, user, "192.0.2.1", "browser one")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = first.CreateSession(ctx, user, "192.0.2.2", "browser two")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	validated, err := second.ValidateSession(ctx, token1)
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if validated.ID != session1.ID || validated.Username != "demo" {
		t.Fatalf("unexpected session: %#v", validated)
	}
	all, err := second.ListSessions(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("sessions = %d, want 2", len(all))
	}
	removed, err := second.DeleteOtherSessions(ctx, user.ID, session1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	all, err = second.ListSessions(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != session1.ID {
		t.Fatalf("unexpected remaining sessions: %#v", all)
	}
}

func TestDatabasePermissionsAndPasswordHash(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.EnsureBootstrapUser(ctx, "admin", "plaintext-secret"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(filepath.Join(dir, "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "" && contains(data, []byte("plaintext-secret")) {
		t.Fatal("database contains plaintext password")
	}
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
