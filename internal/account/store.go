package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	SessionDuration = 30 * 24 * time.Hour
	sessionTouchGap = time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
)

type Store struct {
	db   *sql.DB
	path string
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Session struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"-"`
	Username   string    `json:"username"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func Open(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create account data directory: %w", err)
	}
	path := filepath.Join(dataDir, "accounts.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open account database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure account database: %w", err)
		}
	}

	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure account database: %w", err)
	}
	if err := s.PurgeExpired(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash BLOB NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash BLOB NOT NULL UNIQUE,
	ip_address TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	last_seen_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, expires_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS telegram_config (
	id INTEGER PRIMARY KEY CHECK(id = 1),
	token_cipher TEXT NOT NULL,
	bot_id INTEGER NOT NULL,
	bot_username TEXT NOT NULL,
	notifications_enabled INTEGER NOT NULL DEFAULT 1,
	media_mode TEXT NOT NULL DEFAULT 'photo',
	video_seconds INTEGER NOT NULL DEFAULT 5,
	updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS telegram_chats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	chat_id INTEGER NOT NULL UNIQUE,
	chat_name TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	linked_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS telegram_codes (
	id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	code_hash BLOB NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_telegram_codes_expiry ON telegram_codes(expires_at);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate account database: %w", err)
	}
	return nil
}

func (s *Store) HasUsers(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}

func (s *Store) EnsureBootstrapUser(ctx context.Context, username, password string) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return User{}, errors.New("username and password are required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash bootstrap password: %w", err)
	}
	now := time.Now().UTC().Unix()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO users(username, password_hash, created_at, updated_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash, updated_at=excluded.updated_at
`, username, hash, now, now)
	if err != nil {
		return User{}, fmt.Errorf("upsert bootstrap user: %w", err)
	}
	return s.userByUsername(ctx, username)
}

func (s *Store) Authenticate(ctx context.Context, username, password string) (User, error) {
	username = strings.TrimSpace(username)
	var user User
	var hash []byte
	if err := s.db.QueryRowContext(ctx, "SELECT id, username, password_hash FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&user.ID, &user.Username, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}
		return User{}, fmt.Errorf("lookup user: %w", err)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Store) userByUsername(ctx context.Context, username string) (User, error) {
	var user User
	if err := s.db.QueryRowContext(ctx, "SELECT id, username FROM users WHERE username = ? COLLATE NOCASE", username).Scan(&user.ID, &user.Username); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, user User, ipAddress, userAgent string) (string, Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", Session{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	digest := tokenDigest(token)
	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		return "", Session{}, fmt.Errorf("generate session id: %w", err)
	}
	id := hex.EncodeToString(idBytes)
	now := time.Now().UTC()
	expires := now.Add(SessionDuration)
	ipAddress = limitString(strings.TrimSpace(ipAddress), 128)
	userAgent = limitString(strings.TrimSpace(userAgent), 512)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions(id, user_id, token_hash, ip_address, user_agent, created_at, last_seen_at, expires_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
`, id, user.ID, digest[:], ipAddress, userAgent, now.Unix(), now.Unix(), expires.Unix())
	if err != nil {
		return "", Session{}, fmt.Errorf("create session: %w", err)
	}
	return token, Session{
		ID: id, UserID: user.ID, Username: user.Username, IPAddress: ipAddress, UserAgent: userAgent,
		CreatedAt: now, LastSeenAt: now, ExpiresAt: expires,
	}, nil
}

func (s *Store) ValidateSession(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrSessionNotFound
	}
	digest := tokenDigest(token)
	var session Session
	var createdUnix, seenUnix, expiresUnix int64
	err := s.db.QueryRowContext(ctx, `
SELECT s.id, s.user_id, u.username, s.ip_address, s.user_agent, s.created_at, s.last_seen_at, s.expires_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ?
`, digest[:]).Scan(
		&session.ID, &session.UserID, &session.Username, &session.IPAddress, &session.UserAgent,
		&createdUnix, &seenUnix, &expiresUnix,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("validate session: %w", err)
	}
	session.CreatedAt = time.Unix(createdUnix, 0).UTC()
	session.LastSeenAt = time.Unix(seenUnix, 0).UTC()
	session.ExpiresAt = time.Unix(expiresUnix, 0).UTC()
	now := time.Now().UTC()
	if !session.ExpiresAt.After(now) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", session.ID)
		return Session{}, ErrSessionNotFound
	}
	if now.Sub(session.LastSeenAt) >= sessionTouchGap {
		if _, err := s.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE id = ?", now.Unix(), session.ID); err == nil {
			session.LastSeenAt = now
		}
	}
	return session, nil
}

func (s *Store) DeleteSessionByToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	digest := tokenDigest(token)
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", digest[:])
	return err
}

func (s *Store) ListSessions(ctx context.Context, userID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, s.user_id, u.username, s.ip_address, s.user_agent, s.created_at, s.last_seen_at, s.expires_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.user_id = ? AND s.expires_at > ?
ORDER BY s.last_seen_at DESC
`, userID, time.Now().UTC().Unix())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var item Session
		var createdUnix, seenUnix, expiresUnix int64
		if err := rows.Scan(&item.ID, &item.UserID, &item.Username, &item.IPAddress, &item.UserAgent, &createdUnix, &seenUnix, &expiresUnix); err != nil {
			return nil, err
		}
		item.CreatedAt = time.Unix(createdUnix, 0).UTC()
		item.LastSeenAt = time.Unix(seenUnix, 0).UTC()
		item.ExpiresAt = time.Unix(expiresUnix, 0).UTC()
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, userID int64, sessionID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ? AND user_id = ?", sessionID, userID)
	if err != nil {
		return false, fmt.Errorf("delete session: %w", err)
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (s *Store) DeleteOtherSessions(ctx context.Context, userID int64, currentSessionID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND id <> ?", userID, currentSessionID)
	if err != nil {
		return 0, fmt.Errorf("delete other sessions: %w", err)
	}
	return result.RowsAffected()
}

func (s *Store) PurgeExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("purge expired sessions: %w", err)
	}
	return nil
}

func tokenDigest(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func limitString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
