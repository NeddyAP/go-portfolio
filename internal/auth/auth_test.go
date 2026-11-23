package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NeddyAP/go-portfolio/internal/auth"
)

func TestPasswordCookieCSRFAndRateLimit(t *testing.T) {
	if _, err := auth.HashPassword("short"); err == nil {
		t.Fatal("expected error for short password")
	}

	hash, err := auth.HashPassword("super-secret-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	secret := []byte("0123456789abcdef0123456789abcdef")
	mgr := auth.NewManager("neddy.prasetio@gmail.com", hash, secret, true)

	if !mgr.IsConfigured() {
		t.Fatal("expected manager to be configured")
	}
	if !mgr.VerifyCredentials("NEDDY.PRASETIO@gmail.com", "super-secret-password") {
		t.Fatal("expected valid credentials to verify")
	}
	if mgr.VerifyCredentials("wrong@example.com", "super-secret-password") {
		t.Fatal("wrong email must not verify")
	}
	if mgr.VerifyCredentials("neddy.prasetio@gmail.com", "wrong-password") {
		t.Fatal("wrong password must not verify")
	}

	// Issue session & verify roundtrip
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://neddyap.tech/api/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	sess, err := mgr.IssueSession(rec, req, "neddy.prasetio@gmail.com")
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies (session + csrf), got %d", len(cookies))
	}

	checkReq := httptest.NewRequest(http.MethodPut, "/api/about-me", nil)
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName && !c.Secure {
			t.Fatal("expected Secure flag when X-Forwarded-Proto=https")
		}
		checkReq.AddCookie(c)
	}

	parsedSess, ok := mgr.GetSession(checkReq)
	if !ok || parsedSess.Email != "neddy.prasetio@gmail.com" {
		t.Fatalf("GetSession failed: ok=%v", ok)
	}

	if mgr.ValidateCSRF(checkReq, parsedSess) {
		t.Fatal("expected missing X-CSRF-Token header to fail CSRF check")
	}
	checkReq.Header.Set("X-CSRF-Token", sess.CSRF)
	if !mgr.ValidateCSRF(checkReq, parsedSess) {
		t.Fatal("expected matching X-CSRF-Token header to pass CSRF check")
	}

	// Tampered session cookie rejected
	tamperedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	tamperedReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tampered.payload"})
	if _, ok := mgr.GetSession(tamperedReq); ok {
		t.Fatal("tampered cookie must be rejected")
	}

	// Rate limiter
	ip := "203.0.113.10"
	for i := 0; i < 5; i++ {
		if allowed, _ := mgr.AllowLoginAttempt(ip); !allowed {
			t.Fatalf("attempt %d should be allowed", i)
		}
		mgr.RecordFailedLogin(ip)
	}
	allowed, retryAfter := mgr.AllowLoginAttempt(ip)
	if allowed || retryAfter <= 0*time.Second {
		t.Fatalf("6th attempt should be blocked; allowed=%v retryAfter=%v", allowed, retryAfter)
	}
	mgr.ClearLoginFailures(ip)
	if allowed, _ := mgr.AllowLoginAttempt(ip); !allowed {
		t.Fatal("expected clear to restore attempts")
	}
}
