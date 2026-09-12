package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	keySize         = 32
	defaultPostRoll = 20
)

var ErrNotFound = errors.New("camera not found")

type Camera struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Endpoint        string    `json:"endpoint"`
	Username        string    `json:"username"`
	Password        string    `json:"-"`
	Manufacturer    string    `json:"manufacturer,omitempty"`
	Model           string    `json:"model,omitempty"`
	FirmwareVersion string    `json:"firmware_version,omitempty"`
	ProfileToken    string    `json:"profile_token,omitempty"`
	ProfileName     string    `json:"profile_name,omitempty"`
	RTSPURI         string    `json:"rtsp_uri,omitempty"`
	EventsSupported bool      `json:"events_supported"`
	EventRecording  bool      `json:"event_recording"`
	PostRollSeconds int       `json:"post_roll_seconds"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type persistedCamera struct {
	Camera
	PasswordCipher string `json:"password_cipher,omitempty"`
}

type persistedState struct {
	Version int               `json:"version"`
	Cameras []persistedCamera `json:"cameras"`
}

type Store struct {
	mu        sync.RWMutex
	dir       string
	statePath string
	aead      cipher.AEAD
	cameras   map[string]Camera
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure data directory: %w", err)
	}

	key, err := loadOrCreateKey(filepath.Join(dir, "master.key"))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	s := &Store{
		dir:       dir,
		statePath: filepath.Join(dir, "cameras.json"),
		aead:      aead,
		cameras:   make(map[string]Camera),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != keySize {
			return nil, fmt.Errorf("invalid master key size: %d", len(key))
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, fmt.Errorf("secure master key: %w", err)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read master key: %w", err)
	}

	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}
	return key, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read camera state: %w", err)
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode camera state: %w", err)
	}
	for _, p := range state.Cameras {
		cam := p.Camera
		if p.PasswordCipher != "" {
			plain, err := s.decrypt(p.PasswordCipher)
			if err != nil {
				return fmt.Errorf("decrypt credentials for %s: %w", cam.ID, err)
			}
			cam.Password = plain
		}
		s.cameras[cam.ID] = cam
	}
	return nil
}

func (s *Store) saveLocked() error {
	state := persistedState{Version: 1, Cameras: make([]persistedCamera, 0, len(s.cameras))}
	ids := make([]string, 0, len(s.cameras))
	for id := range s.cameras {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		cam := s.cameras[id]
		p := persistedCamera{Camera: cam}
		p.Camera.Password = ""
		if cam.Password != "" {
			ciphertext, err := s.encrypt(cam.Password)
			if err != nil {
				return err
			}
			p.PasswordCipher = ciphertext
		}
		state.Cameras = append(state.Cameras, p)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode camera state: %w", err)
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write camera state: %w", err)
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		return fmt.Errorf("publish camera state: %w", err)
	}
	return os.Chmod(s.statePath, 0o600)
}

func (s *Store) encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := s.aead.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, sealed...)
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (s *Store) decrypt(encoded string) (string, error) {
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode cipher text: %w", err)
	}
	if len(payload) < s.aead.NonceSize() {
		return "", errors.New("cipher text too short")
	}
	nonce, ciphertext := payload[:s.aead.NonceSize()], payload[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt cipher text: %w", err)
	}
	return string(plain), nil
}

func (s *Store) Add(cam Camera) (Camera, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cam.ID == "" {
		id, err := newID()
		if err != nil {
			return Camera{}, err
		}
		cam.ID = id
	}
	now := time.Now().UTC()
	if cam.PostRollSeconds <= 0 {
		cam.PostRollSeconds = defaultPostRoll
	}
	cam.CreatedAt = now
	cam.UpdatedAt = now
	s.cameras[cam.ID] = cam
	if err := s.saveLocked(); err != nil {
		delete(s.cameras, cam.ID)
		return Camera{}, err
	}
	return cam, nil
}

func (s *Store) List() []Camera {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Camera, 0, len(s.cameras))
	for _, cam := range s.cameras {
		cam.Password = ""
		out = append(out, cam)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) Get(id string) (Camera, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cam, ok := s.cameras[id]
	if !ok {
		return Camera{}, ErrNotFound
	}
	return cam, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam, ok := s.cameras[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.cameras, id)
	if err := s.saveLocked(); err != nil {
		s.cameras[id] = cam
		return err
	}
	return nil
}

func (s *Store) SetRecording(id string, enabled bool, postRollSeconds int) (Camera, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cam, ok := s.cameras[id]
	if !ok {
		return Camera{}, ErrNotFound
	}
	if postRollSeconds <= 0 {
		postRollSeconds = defaultPostRoll
	}
	cam.EventRecording = enabled
	cam.PostRollSeconds = postRollSeconds
	cam.UpdatedAt = time.Now().UTC()
	s.cameras[id] = cam
	if err := s.saveLocked(); err != nil {
		return Camera{}, err
	}
	cam.Password = ""
	return cam, nil
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
