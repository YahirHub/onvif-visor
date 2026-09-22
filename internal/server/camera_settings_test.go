package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"onvif-visor/internal/store"
)

func TestHandleCameraNameUpdatesStore(t *testing.T) {
	cameraStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cam, err := cameraStore.Add(store.Camera{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: cameraStore}

	req := httptest.NewRequest(http.MethodPut, "/api/cameras/"+cam.ID+"/name", strings.NewReader(`{"name":"Entrada principal"}`))
	req.SetPathValue("id", cam.ID)
	rec := httptest.NewRecorder()
	app.handleCameraName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loaded, err := cameraStore.Get(cam.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "Entrada principal" {
		t.Fatalf("name=%q", loaded.Name)
	}
}

func TestHandleCameraNameRejectsBlankName(t *testing.T) {
	cameraStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cam, err := cameraStore.Add(store.Camera{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: cameraStore}

	req := httptest.NewRequest(http.MethodPut, "/api/cameras/"+cam.ID+"/name", strings.NewReader(`{"name":"   "}`))
	req.SetPathValue("id", cam.ID)
	rec := httptest.NewRecorder()
	app.handleCameraName(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
