package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// session represents an authenticated user session.
type session struct {
	token     string
	expiresAt time.Time
}

// Manager handles password-based authentication with session cookies.
type Manager struct {
	password string
	sessions map[string]*session
	mu       sync.RWMutex
}

// NewManager creates a new auth manager with the given password.
func NewManager(password string) *Manager {
	return &Manager{
		password: password,
		sessions: make(map[string]*session),
	}
}

// CheckPassword validates the provided password.
func (m *Manager) CheckPassword(pw string) bool {
	return pw == m.password
}

// CreateSession generates a new session token and stores it.
func (m *Manager) CreateSession() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	tokenStr := hex.EncodeToString(token)
	m.mu.Lock()
	m.sessions[tokenStr] = &session{
		token:     tokenStr,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	m.mu.Unlock()

	return tokenStr, nil
}

// ValidateSession checks if a session token is valid and not expired.
func (m *Manager) ValidateSession(token string) bool {
	m.mu.RLock()
	sess, ok := m.sessions[token]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	if time.Now().After(sess.expiresAt) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return false
	}
	return true
}

// SetSessionCookie sets the authentication cookie on the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})
}

// GetSessionToken extracts the session token from the request cookie.
func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("__session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// Middleware returns an HTTP middleware that protects routes with authentication.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetSessionToken(r)
		if !m.ValidateSession(token) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
