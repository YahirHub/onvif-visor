package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreEncryptsAndReloadsPassword(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	added, err := first.Add(Camera{
		Name:     "Entrada",
		Endpoint: "http://192.0.2.10/onvif/device_service",
		Username: "operator",
		Password: "super-secret-value",
		RTSPURI:  "rtsp://192.0.2.10/stream1",
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	state, err := os.ReadFile(filepath.Join(dir, "cameras.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(state, []byte("super-secret-value")) {
		t.Fatal("camera state contains plaintext password")
	}

	second, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	loaded, err := second.Get(added.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Password != "super-secret-value" {
		t.Fatalf("Password = %q, want original secret", loaded.Password)
	}
	if loaded.RTSPURI != "rtsp://192.0.2.10/stream1" {
		t.Fatalf("RTSPURI = %q", loaded.RTSPURI)
	}
}

func TestListNeverReturnsPassword(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Add(Camera{Name: "Cam", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(list))
	}
	if list[0].Password != "" {
		t.Fatal("List() exposed camera password")
	}
}
