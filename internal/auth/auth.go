package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrRateLimited = errors.New("too many login attempts")

const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64MB
	argonThreads = 4
	argonKeyLen  = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return "argon2id$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

type Session struct {
	Token     string
	UserID    int
	CSRF      string
	ExpiresAt time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Login(username, password string) (*Session, error) {
	var id int
	var hash string
	err := s.db.QueryRow(`SELECT id, password_hash FROM users WHERE username = ?`, username).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !VerifyPassword(password, hash) {
		return nil, ErrInvalidCredentials
	}

	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(7 * 24 * time.Hour)

	_, err = s.db.Exec(`INSERT INTO sessions (token, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`,
		token, id, csrf, expires)
	if err != nil {
		return nil, err
	}
	return &Session{Token: token, UserID: id, CSRF: csrf, ExpiresAt: expires}, nil
}

func (s *Store) Validate(token string) (*Session, error) {
	var sess Session
	var userID int
	err := s.db.QueryRow(`SELECT user_id, csrf_token, expires_at FROM sessions WHERE token = ?`, token).
		Scan(&userID, &sess.CSRF, &sess.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil, errors.New("session expired")
	}
	sess.Token = token
	sess.UserID = userID
	return &sess, nil
}

func (s *Store) Logout(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// --- login rate limiting (in-memory sliding window per IP) ---

type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func NewLimiter() *Limiter { return &Limiter{hits: make(map[string][]time.Time)} }

func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	window := now.Add(-1 * time.Minute)

	hits := l.hits[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(window) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 5 {
		l.hits[ip] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[ip] = kept
	return true
}