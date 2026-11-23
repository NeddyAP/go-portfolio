package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "portfolio_session"
	CSRFCookieName    = "portfolio_csrf"
	SessionDuration   = 7 * 24 * time.Hour

	rateLimitWindow   = 15 * time.Minute
	rateLimitMaxTries = 5
)

// dummyBcryptHash ensures login timing stays uniform even if email does not match.
var dummyBcryptHash = []byte("$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234")

// Session stores authenticated admin session claims.
type Session struct {
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
	CSRF  string `json:"csrf"`
}

// Manager handles admin authentication, signed session cookies, CSRF validation, and rate limiting.
type Manager struct {
	adminEmail string
	adminHash  []byte
	secret     []byte
	trustProxy bool

	mu       sync.Mutex
	failures map[string][]time.Time
	nowFn    func() time.Time
}

// NewManager creates an auth Manager. If adminHash is empty, admin login is disabled.
func NewManager(adminEmail, adminHash string, secret []byte, trustProxy bool) *Manager {
	return &Manager{
		adminEmail: strings.ToLower(strings.TrimSpace(adminEmail)),
		adminHash:  []byte(strings.TrimSpace(adminHash)),
		secret:     secret,
		trustProxy: trustProxy,
		failures:   make(map[string][]time.Time),
		nowFn:      time.Now,
	}
}

// IsConfigured reports whether an admin password hash has been provided.
func (m *Manager) IsConfigured() bool {
	return len(m.adminHash) > 0 && m.adminEmail != ""
}

// HashPassword generates a bcrypt hash from a plaintext password.
func HashPassword(plain string) (string, error) {
	if len(plain) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyCredentials checks the provided email and password in constant time.
func (m *Manager) VerifyCredentials(email, password string) bool {
	if !m.IsConfigured() {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
		return false
	}

	emailMatch := hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(email))), []byte(m.adminEmail))
	targetHash := dummyBcryptHash
	if emailMatch {
		targetHash = m.adminHash
	}
	err := bcrypt.CompareHashAndPassword(targetHash, []byte(password))
	return emailMatch && err == nil
}

// IssueSession creates signed session and CSRF cookies on the response.
func (m *Manager) IssueSession(w http.ResponseWriter, r *http.Request, email string) (*Session, error) {
	csrfBytes := make([]byte, 24)
	if _, err := rand.Read(csrfBytes); err != nil {
		return nil, err
	}
	csrf := base64.RawURLEncoding.EncodeToString(csrfBytes)
	sess := &Session{
		Email: email,
		Exp:   m.nowFn().Add(SessionDuration).Unix(),
		CSRF:  csrf,
	}

	token, err := m.signSession(sess)
	if err != nil {
		return nil, err
	}

	secure := m.IsRequestHTTPS(r)
	maxAge := int(SessionDuration.Seconds())

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrf,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return sess, nil
}

// ClearSession removes the session and CSRF cookies.
func (m *Manager) ClearSession(w http.ResponseWriter, r *http.Request) {
	secure := m.IsRequestHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSession parses and validates the signed session cookie from the request.
func (m *Manager) GetSession(r *http.Request) (*Session, bool) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		return nil, false
	}
	sess, err := m.verifySession(c.Value)
	if err != nil {
		return nil, false
	}
	if m.nowFn().Unix() >= sess.Exp {
		return nil, false
	}
	return sess, true
}

// ValidateCSRF checks that the X-CSRF-Token header matches both the CSRF cookie and the session's CSRF claim.
func (m *Manager) ValidateCSRF(r *http.Request, sess *Session) bool {
	if sess == nil || sess.CSRF == "" {
		return false
	}
	header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || header == "" || cookie.Value == "" {
		return false
	}
	return hmac.Equal([]byte(header), []byte(sess.CSRF)) &&
		hmac.Equal([]byte(cookie.Value), []byte(sess.CSRF))
}

// AllowLoginAttempt checks whether the IP is within the allowed failed login threshold.
func (m *Manager) AllowLoginAttempt(ip string) (bool, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFn()
	cutoff := now.Add(-rateLimitWindow)
	recent := pruneTimes(m.failures[ip], cutoff)
	if len(recent) == 0 {
		delete(m.failures, ip)
		return true, 0
	}
	m.failures[ip] = recent
	if len(recent) >= rateLimitMaxTries {
		retryAfter := recent[0].Add(rateLimitWindow).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}
	return true, 0
}

// RecordFailedLogin records a failed login attempt for rate limiting.
func (m *Manager) RecordFailedLogin(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFn()
	cutoff := now.Add(-rateLimitWindow)
	recent := pruneTimes(m.failures[ip], cutoff)
	recent = append(recent, now)
	m.failures[ip] = recent

	if len(m.failures) > 256 {
		for k, times := range m.failures {
			if len(pruneTimes(times, cutoff)) == 0 {
				delete(m.failures, k)
			}
		}
	}
}

// ClearLoginFailures clears failed attempts for an IP after successful login.
func (m *Manager) ClearLoginFailures(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, ip)
}

// IsRequestHTTPS detects whether the request arrived over HTTPS directly or via trusted proxy.
func (m *Manager) IsRequestHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if m.trustProxy {
		proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
		if proto == "https" {
			return true
		}
		if strings.Contains(strings.ToLower(r.Header.Get("CF-Visitor")), `"https"`) {
			return true
		}
	}
	return false
}

func (m *Manager) signSession(sess *Session) (string, error) {
	payloadBytes, err := json.Marshal(sess)
	if err != nil {
		return "", err
	}
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := m.computeMAC(payloadEnc)
	return payloadEnc + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (m *Manager) verifySession(token string) (*Session, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("malformed token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	expected := m.computeMAC(parts[0])
	if !hmac.Equal(sig, expected) {
		return nil, errors.New("invalid signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(payloadBytes, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (m *Manager) computeMAC(data string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}

func pruneTimes(times []time.Time, cutoff time.Time) []time.Time {
	if len(times) == 0 {
		return nil
	}
	var out []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}
