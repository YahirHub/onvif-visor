package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"

	"onvif-visor/internal/media"
)

// errRecordingProtected frena cualquier borrado de una grabación protegida:
// ni a mano, ni por lote, ni la limpieza automática de retención.
var errRecordingProtected = errors.New("grabación protegida")

func protectedKey(cameraID, name string) string { return cameraID + "/" + name }

// loadProtected lee el conjunto persistido en <DataDir>/protected_recordings.json.
// Si el archivo no existe o está dañado, se empieza vacío: proteger es una
// ayuda, no un candado del que dependa el arranque.
func (a *App) loadProtected() {
	a.protectedSet = make(map[string]bool)
	if a.protectedPath == "" {
		return
	}
	data, err := os.ReadFile(a.protectedPath)
	if err != nil {
		return
	}
	var keys []string
	if json.Unmarshal(data, &keys) != nil {
		return
	}
	for _, key := range keys {
		a.protectedSet[key] = true
	}
}

func (a *App) isProtected(cameraID, name string) bool {
	a.protectedMu.Lock()
	defer a.protectedMu.Unlock()
	return a.protectedSet[protectedKey(cameraID, name)]
}

func (a *App) setProtected(cameraID, name string, protected bool) error {
	a.protectedMu.Lock()
	defer a.protectedMu.Unlock()
	if a.protectedSet == nil {
		a.protectedSet = make(map[string]bool)
	}
	key := protectedKey(cameraID, name)
	if protected {
		a.protectedSet[key] = true
	} else {
		delete(a.protectedSet, key)
	}
	if a.protectedPath == "" {
		return nil
	}
	keys := make([]string, 0, len(a.protectedSet))
	for k := range a.protectedSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	tmp := a.protectedPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.protectedPath)
}

func (a *App) handleRecordingProtect(w http.ResponseWriter, r *http.Request) {
	cameraID, name := r.PathValue("cameraID"), r.PathValue("name")
	if _, err := a.store.Get(cameraID); err != nil {
		writeError(w, http.StatusNotFound, "Cámara no encontrada.")
		return
	}
	if _, err := a.media.RecordingPath(cameraID, name); err != nil {
		if errors.Is(err, media.ErrRecordingNotFound) {
			writeError(w, http.StatusNotFound, "Grabación no encontrada.")
			return
		}
		writeError(w, http.StatusInternalServerError, "No se pudo abrir la grabación.")
		return
	}
	var req struct {
		Protected bool `json:"protected"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Petición inválida.")
		return
	}
	if err := a.setProtected(cameraID, name, req.Protected); err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo guardar la protección.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"protected": req.Protected, "name": name})
}
