package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameTaken = errors.New("username already exists")

type ProfileUpdateResult struct {
	User                User
	PasswordChanged     bool
	OtherSessionsClosed int64
}

func (s *Store) UserByID(ctx context.Context, userID int64) (User, error) {
	var user User
	if err := s.db.QueryRowContext(ctx, "SELECT id, username FROM users WHERE id = ?", userID).Scan(&user.ID, &user.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, fmt.Errorf("lookup user by id: %w", err)
	}
	return user, nil
}

func (s *Store) UpdateProfile(ctx context.Context, userID int64, currentSessionID, currentPassword, username, newPassword string) (ProfileUpdateResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || currentPassword == "" {
		return ProfileUpdateResult{}, ErrInvalidCredentials
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProfileUpdateResult{}, fmt.Errorf("begin profile update: %w", err)
	}
	defer tx.Rollback()

	var current User
	var passwordHash []byte
	if err := tx.QueryRowContext(ctx, "SELECT id, username, password_hash FROM users WHERE id = ?", userID).Scan(&current.ID, &current.Username, &passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProfileUpdateResult{}, ErrInvalidCredentials
		}
		return ProfileUpdateResult{}, fmt.Errorf("load profile: %w", err)
	}
	if bcrypt.CompareHashAndPassword(passwordHash, []byte(currentPassword)) != nil {
		return ProfileUpdateResult{}, ErrInvalidCredentials
	}

	var conflictID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ? COLLATE NOCASE AND id <> ?", username, userID).Scan(&conflictID)
	if err == nil {
		return ProfileUpdateResult{}, ErrUsernameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ProfileUpdateResult{}, fmt.Errorf("check username availability: %w", err)
	}

	passwordChanged := newPassword != ""
	nextHash := passwordHash
	if passwordChanged {
		nextHash, err = bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return ProfileUpdateResult{}, fmt.Errorf("hash new password: %w", err)
		}
	}

	result, err := tx.ExecContext(ctx, "UPDATE users SET username = ?, password_hash = ?, updated_at = ? WHERE id = ?", username, nextHash, time.Now().UTC().Unix(), userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ProfileUpdateResult{}, ErrUsernameTaken
		}
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ProfileUpdateResult{}, ErrInvalidCredentials
	}

	var closed int64
	if passwordChanged {
		deleteResult, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND id <> ?", userID, currentSessionID)
		if err != nil {
			return ProfileUpdateResult{}, fmt.Errorf("revoke other sessions after password change: %w", err)
		}
		closed, _ = deleteResult.RowsAffected()
	}

	if err := tx.Commit(); err != nil {
		return ProfileUpdateResult{}, fmt.Errorf("commit profile update: %w", err)
	}
	return ProfileUpdateResult{
		User:                User{ID: userID, Username: username},
		PasswordChanged:     passwordChanged,
		OtherSessionsClosed: closed,
	}, nil
}
