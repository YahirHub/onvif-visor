package account

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	TelegramCodeDuration        = 10 * time.Minute
	TelegramVideoMinSeconds     = 3
	TelegramVideoMaxSeconds     = 60
	TelegramVideoDefaultSeconds = 5
)

type TelegramConfig struct {
	TokenCipher          string    `json:"-"`
	BotID                int64     `json:"bot_id"`
	BotUsername          string    `json:"bot_username"`
	NotificationsEnabled bool      `json:"notifications_enabled"`
	MediaMode            string    `json:"media_mode"`
	VideoSeconds         int       `json:"video_seconds"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type TelegramChat struct {
	ID       int64     `json:"id"`
	UserID   int64     `json:"-"`
	ChatID   int64     `json:"chat_id"`
	ChatName string    `json:"chat_name"`
	Enabled  bool      `json:"enabled"`
	LinkedAt time.Time `json:"linked_at"`
}

func (s *Store) GetTelegramConfig(ctx context.Context) (TelegramConfig, bool, error) {
	var cfg TelegramConfig
	var enabled int
	var updated int64
	err := s.db.QueryRowContext(ctx, `
SELECT token_cipher, bot_id, bot_username, notifications_enabled, media_mode, video_seconds, updated_at
FROM telegram_config WHERE id = 1
`).Scan(&cfg.TokenCipher, &cfg.BotID, &cfg.BotUsername, &enabled, &cfg.MediaMode, &cfg.VideoSeconds, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return TelegramConfig{}, false, nil
	}
	if err != nil {
		return TelegramConfig{}, false, fmt.Errorf("get telegram config: %w", err)
	}
	cfg.NotificationsEnabled = enabled != 0
	cfg.UpdatedAt = time.Unix(updated, 0).UTC()
	return cfg, true, nil
}

func (s *Store) SaveTelegramConfig(ctx context.Context, cfg TelegramConfig) error {
	if cfg.MediaMode != "photo" && cfg.MediaMode != "video" {
		return errors.New("invalid telegram media mode")
	}
	if cfg.VideoSeconds < TelegramVideoMinSeconds || cfg.VideoSeconds > TelegramVideoMaxSeconds {
		return errors.New("invalid telegram video duration")
	}
	enabled := 0
	if cfg.NotificationsEnabled {
		enabled = 1
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO telegram_config(id, token_cipher, bot_id, bot_username, notifications_enabled, media_mode, video_seconds, updated_at)
VALUES(1, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	token_cipher=excluded.token_cipher,
	bot_id=excluded.bot_id,
	bot_username=excluded.bot_username,
	notifications_enabled=excluded.notifications_enabled,
	media_mode=excluded.media_mode,
	video_seconds=excluded.video_seconds,
	updated_at=excluded.updated_at
`, cfg.TokenCipher, cfg.BotID, cfg.BotUsername, enabled, cfg.MediaMode, cfg.VideoSeconds, now.Unix())
	if err != nil {
		return fmt.Errorf("save telegram config: %w", err)
	}
	return nil
}

func (s *Store) UpdateTelegramSettings(ctx context.Context, notifications bool, mediaMode string, videoSeconds int) error {
	if mediaMode != "photo" && mediaMode != "video" {
		return errors.New("invalid telegram media mode")
	}
	if videoSeconds < TelegramVideoMinSeconds || videoSeconds > TelegramVideoMaxSeconds {
		return errors.New("invalid telegram video duration")
	}
	enabled := 0
	if notifications {
		enabled = 1
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE telegram_config SET notifications_enabled=?, media_mode=?, video_seconds=?, updated_at=? WHERE id=1
`, enabled, mediaMode, videoSeconds, time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("update telegram settings: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteTelegramConfig(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM telegram_codes"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM telegram_chats"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM telegram_config WHERE id=1"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListTelegramChats(ctx context.Context) ([]TelegramChat, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, chat_id, chat_name, enabled, linked_at FROM telegram_chats ORDER BY linked_at DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list telegram chats: %w", err)
	}
	defer rows.Close()
	var out []TelegramChat
	for rows.Next() {
		var item TelegramChat
		var enabled int
		var linked int64
		if err := rows.Scan(&item.ID, &item.UserID, &item.ChatID, &item.ChatName, &enabled, &linked); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.LinkedAt = time.Unix(linked, 0).UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) TelegramChatLinked(ctx context.Context, chatID int64) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM telegram_chats WHERE chat_id=? AND enabled=1", chatID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) LinkTelegramChat(ctx context.Context, userID, chatID int64, chatName string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO telegram_chats(user_id, chat_id, chat_name, enabled, linked_at)
VALUES(?, ?, ?, 1, ?)
ON CONFLICT(chat_id) DO UPDATE SET user_id=excluded.user_id, chat_name=excluded.chat_name, enabled=1, linked_at=excluded.linked_at
`, userID, chatID, limitString(chatName, 160), time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("link telegram chat: %w", err)
	}
	return nil
}

func (s *Store) CreateTelegramCode(ctx context.Context, userID int64) (string, time.Time, error) {
	var random [2]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", time.Time{}, err
	}
	value := (int(random[0])<<8 | int(random[1])) % 10000
	code := fmt.Sprintf("%04d", value)
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.MinCost)
	if err != nil {
		return "", time.Time{}, err
	}
	idRaw := make([]byte, 8)
	if _, err := rand.Read(idRaw); err != nil {
		return "", time.Time{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(TelegramCodeDuration)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM telegram_codes WHERE user_id=? OR expires_at <= ?", userID, now.Unix()); err != nil {
		return "", time.Time{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_codes(id, user_id, code_hash, created_at, expires_at) VALUES(?, ?, ?, ?, ?)`, hex.EncodeToString(idRaw), userID, hash, now.Unix(), expires.Unix()); err != nil {
		return "", time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", time.Time{}, err
	}
	return code, expires, nil
}

func (s *Store) ConsumeTelegramCode(ctx context.Context, code string) (int64, bool, error) {
	now := time.Now().UTC().Unix()
	rows, err := s.db.QueryContext(ctx, "SELECT id, user_id, code_hash FROM telegram_codes WHERE expires_at > ?", now)
	if err != nil {
		return 0, false, err
	}
	type candidate struct {
		id     string
		userID int64
		hash   []byte
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.userID, &c.hash); err != nil {
			rows.Close()
			return 0, false, err
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	for _, c := range candidates {
		if bcrypt.CompareHashAndPassword(c.hash, []byte(code)) == nil {
			result, err := s.db.ExecContext(ctx, "DELETE FROM telegram_codes WHERE id=?", c.id)
			if err != nil {
				return 0, false, err
			}
			n, _ := result.RowsAffected()
			return c.userID, n > 0, nil
		}
	}
	return 0, false, nil
}
