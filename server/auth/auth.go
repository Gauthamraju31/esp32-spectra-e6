package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/Gauthamraju31/esp32-spectra-e6/server/config"
)

// session represents an authenticated user session.
type session struct {
	token     string
	username  string
	role      string
	expiresAt time.Time
}

// Manager handles password-based authentication with session cookies.
type Manager struct {
	users    []config.User
	sessions map[string]*session
	mu       sync.RWMutex
}

// NewManager creates a new auth manager with the given users list.
func NewManager(users []config.User) *Manager {
	return &Manager{
		users:    users,
		sessions: make(map[string]*session),
	}
}

// CheckCredentials validates the provided username and password and returns the user if matched.
func (m *Manager) CheckCredentials(username, pw string) (*config.User, bool) {
	for _, u := range m.users {
		if u.Username == username && u.Password == pw {
			return &u, true
		}
	}
	// Fallback to check if they typed the password in username or something? No, keep it strict.
	return nil, false
}

// CreateSession generates a new session token and stores it.
func (m *Manager) CreateSession(user *config.User) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	tokenStr := hex.EncodeToString(token)
	m.mu.Lock()
	m.sessions[tokenStr] = &session{
		token:     tokenStr,
		username:  user.Username,
		role:      user.Role,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	m.mu.Unlock()

	return tokenStr, nil
}

// GetSessionRole returns the role of the user associated with the token.
func (m *Manager) GetSessionRole(token string) string {
	m.mu.RLock()
	sess, ok := m.sessions[token]
	m.mu.RUnlock()

	if !ok {
		return ""
	}
	if time.Now().After(sess.expiresAt) {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
		return ""
	}
	return sess.role
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
