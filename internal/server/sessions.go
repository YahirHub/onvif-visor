package server

import (
	"net/http"
	"strings"
	"time"
)

type sessionView struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

func (a *App) handleSessions(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	items, err := a.accounts.ListSessions(r.Context(), current.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron consultar las sesiones.")
		return
	}
	out := make([]sessionView, 0, len(items))
	for _, item := range items {
		out = append(out, sessionView{
			ID: item.ID, Username: item.Username, IPAddress: item.IPAddress, UserAgent: item.UserAgent,
			CreatedAt: item.CreatedAt, LastSeenAt: item.LastSeenAt, ExpiresAt: item.ExpiresAt,
			Current: item.ID == current.ID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "Sesión inválida.")
		return
	}
	deleted, err := a.accounts.DeleteSession(r.Context(), current.UserID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo cerrar la sesión.")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "Sesión no encontrada.")
		return
	}
	if id == current.ID {
		clearSessionCookie(w, r)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteOtherSessions(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	deleted, err := a.accounts.DeleteOtherSessions(r.Context(), current.UserID, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron cerrar las otras sesiones.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": deleted})
}
