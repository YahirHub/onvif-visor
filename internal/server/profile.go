package server

import (
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"onvif-visor/internal/account"
)

type profileView struct {
	Username string `json:"username"`
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	user, err := a.accounts.UserByID(r.Context(), current.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo consultar el perfil.")
		return
	}
	writeJSON(w, http.StatusOK, profileView{Username: user.Username})
}

func (a *App) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	current, ok := currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "La sesión no es válida.")
		return
	}
	var req struct {
		Username        string `json:"username"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Datos de perfil inválidos.")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if err := validateProfileUsername(req.Username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "Escribe tu contraseña actual para confirmar los cambios.")
		return
	}
	if len(req.CurrentPassword) > 256 {
		writeError(w, http.StatusBadRequest, "La contraseña actual es demasiado larga.")
		return
	}
	if req.NewPassword != "" {
		if len(req.NewPassword) < 8 {
			writeError(w, http.StatusBadRequest, "La contraseña nueva debe tener al menos 8 caracteres.")
			return
		}
		if len(req.NewPassword) > 128 {
			writeError(w, http.StatusBadRequest, "La contraseña nueva es demasiado larga.")
			return
		}
	}

	result, err := a.accounts.UpdateProfile(r.Context(), current.UserID, current.ID, req.CurrentPassword, req.Username, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrInvalidCredentials):
			writeError(w, http.StatusForbidden, "La contraseña actual no es correcta.")
		case errors.Is(err, account.ErrUsernameTaken):
			writeError(w, http.StatusConflict, "Ese nombre de usuario ya está en uso.")
		default:
			writeError(w, http.StatusInternalServerError, "No se pudo actualizar el perfil.")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":              result.User.Username,
		"password_changed":      result.PasswordChanged,
		"other_sessions_closed": result.OtherSessionsClosed,
	})
}

func validateProfileUsername(username string) error {
	count := utf8.RuneCountInString(username)
	if count < 3 || count > 64 {
		return errors.New("El usuario debe tener entre 3 y 64 caracteres.")
	}
	for _, r := range username {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			continue
		}
		return errors.New("El usuario sólo puede usar letras, números, punto, guion y guion bajo.")
	}
	return nil
}
