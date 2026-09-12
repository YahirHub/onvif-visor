package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeIfDestinationEmptyMigratesOnce(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "secret.txt"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyTreeIfDestinationEmpty(source, destination, 0o700, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "nested", "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Fatalf("copied data = %q", data)
	}
	info, err := os.Stat(filepath.Join(destination, "nested", "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}

	if err := os.WriteFile(filepath.Join(source, "nested", "secret.txt"), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyTreeIfDestinationEmpty(source, destination, 0o700, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(destination, "nested", "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Fatalf("existing destination was overwritten: %q", data)
	}
}

func TestCopyTreeRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyTreeIfDestinationEmpty(source, destination, 0o700, 0o600); err == nil {
		t.Fatal("expected symlink migration to fail")
	}
}
