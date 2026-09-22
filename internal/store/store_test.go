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

func TestSetNamePersists(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cam, err := first.Add(Camera{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := first.SetName(cam.ID, "Entrada principal")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Entrada principal" {
		t.Fatalf("updated name = %q", updated.Name)
	}
	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := second.Get(cam.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Entrada principal" {
		t.Fatalf("persisted name = %q", loaded.Name)
	}
}

func TestRecordingAndRetentionSettingsPersist(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cam, err := first.Add(Camera{Name: "Cocina"})
	if err != nil {
		t.Fatal(err)
	}
	if !cam.AutoRetention || cam.RetentionDiskPercent != defaultRetentionDiskPercent {
		t.Fatalf("new camera retention defaults = enabled:%v percent:%d", cam.AutoRetention, cam.RetentionDiskPercent)
	}
	if _, err := first.SetPersonRecording(cam.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := first.SetRecording(cam.ID, true, 45); err != nil {
		t.Fatal(err)
	}
	if _, err := first.SetRetention(cam.ID, true, 88); err != nil {
		t.Fatal(err)
	}

	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := second.Get(cam.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.PersonRecording || !loaded.EventRecording || loaded.PostRollSeconds != 45 {
		t.Fatalf("recording settings = %#v", loaded)
	}
	if !loaded.AutoRetention || loaded.RetentionDiskPercent != 88 {
		t.Fatalf("retention settings = enabled:%v percent:%d", loaded.AutoRetention, loaded.RetentionDiskPercent)
	}
}

func TestSchedulePersists(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cam, err := first.Add(Camera{Name: "Horario"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = first.SetSchedule(cam.ID, true, []int{1, 2, 3, 4, 5}, "08:30", "18:15", "America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := second.Get(cam.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ScheduleEnabled || loaded.ScheduleStart != "08:30" || loaded.ScheduleEnd != "18:15" || loaded.ScheduleTimezone != "America/Mexico_City" {
		t.Fatalf("unexpected schedule: %#v", loaded)
	}
	if len(loaded.ScheduleDays) != 5 || loaded.ScheduleDays[0] != 1 || loaded.ScheduleDays[4] != 5 {
		t.Fatalf("schedule days = %#v", loaded.ScheduleDays)
	}
}
