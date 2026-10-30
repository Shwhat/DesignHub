// Package auth hashes passwords, issues session tokens and limits sign-in
// attempts.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (OWASP recommended minimums or higher).
const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// MinPasswordLength is the shortest password accepted.
const MinPasswordLength = 10

// HashPassword returns an encoded Argon2id hash.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// dummyHash is checked when the user does not exist, so a wrong username
// takes as long as a wrong password.
var dummyHash, _ = HashPassword("designhub-timing-equalizer")

// CheckPassword reports whether pw matches the encoded hash. An empty hash
// never matches but still spends the same time.
func CheckPassword(encoded, pw string) bool {
	if encoded == "" {
		encoded = dummyHash
		pw += "\x00"
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[4])
	want, err2 := enc.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ValidatePassword checks the password rules.
func ValidatePassword(pw, username string) error {
	if len([]rune(pw)) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if username != "" && strings.EqualFold(pw, username) {
		return errors.New("password must not be the username")
	}
	return nil
}

// NewToken returns a random token and its SHA-256 hash for storage.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken hashes a session token for lookup.
func HashToken(token string) []byte {
	s := sha256.Sum256([]byte(token))
	return s[:]
}

// Limiter blocks sign-in after repeated failures for a username or address.
type Limiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	Max      int
	Window   time.Duration
}

// NewLimiter allows max failures per window for each key.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{failures: map[string][]time.Time{}, Max: max, Window: window}
}

func (l *Limiter) recent(key string, now time.Time) []time.Time {
	var keep []time.Time
	for _, t := range l.failures[key] {
		if now.Sub(t) < l.Window {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		delete(l.failures, key)
	} else {
		l.failures[key] = keep
	}
	return keep
}

// Blocked reports whether any of the keys has reached the limit.
func (l *Limiter) Blocked(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		if len(l.recent(k, now)) >= l.Max {
			return true
		}
	}
	return false
}

// Fail records a failed attempt for each key.
func (l *Limiter) Fail(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, k := range keys {
		l.failures[k] = append(l.recent(k, now), now)
	}
}

// Reset clears the failures for the keys after a successful sign-in.
func (l *Limiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		delete(l.failures, k)
	}
}
