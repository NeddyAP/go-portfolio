package handlers_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeddyAP/go-portfolio/internal/auth"
	"github.com/NeddyAP/go-portfolio/internal/cache"
	"github.com/NeddyAP/go-portfolio/internal/config"
	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/github"
	"github.com/NeddyAP/go-portfolio/internal/handlers"
	"github.com/NeddyAP/go-portfolio/web"
)

func setupTestServer(t *testing.T) (http.Handler, *db.Store) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SeedIfEmpty(context.Background()); err != nil {
		t.Fatalf("SeedIfEmpty: %v", err)
	}

	pwHash, err := auth.HashPassword("admin-secret-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	cfg := &config.Config{
		Host:              "127.0.0.1",
		Port:              "8080",
		SiteURL:           "https://neddyap.tech",
		DataDir:           dataDir,
		AdminEmail:        "neddy.prasetio@gmail.com",
		AdminPasswordHash: pwHash,
		SessionSecret:     []byte("0123456789abcdef0123456789abcdef"),
		TrustProxy:        true,
		MaxUploadBytes:    5 * 1024 * 1024,
		GitHubAPIURL:      "http://127.0.0.1:0", // unreachable on purpose to verify non-blocking
	}

	authMgr := auth.NewManager(cfg.AdminEmail, cfg.AdminPasswordHash, cfg.SessionSecret, cfg.TrustProxy)
	pageCache := cache.New()
	ghSvc := github.NewService(cfg.GitHubAPIURL, pageCache.Invalidate)

	srv, err := handlers.NewServer(cfg, store, authMgr, pageCache, ghSvc, web.TemplatesFS, web.StaticFS)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Routes(), store
}

func TestPublicHomeSEORoutesAndCache(t *testing.T) {
	h, _ := setupTestServer(t)

	// 1. GET / with gzip
	req := httptest.NewRequest(http.MethodGet, "http://neddyap.tech/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", rec.Header().Get("Content-Encoding"))
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing Content-Security-Policy header")
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag header")
	}

	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	bodyBytes, err := io.ReadAll(gr)
	_ = gr.Close()
	if err != nil {
		t.Fatalf("read gzipped body: %v", err)
	}
	html := string(bodyBytes)

	for _, want := range []string{
		"Neddy Avgha Prasetio",
		"E-LGCertified",
		"SIMKAN",
		"DICAS &amp; DICSS 9th Djuanda",
		"PT. Helios Informatika Nusantara",
		"https://neddyap.tech/",
		"Bogor, Jawa Barat",
		"project-featured-card",
		"portfolio-filters",
		"browser-bar",
		"e-lgcertified.com",
		"project-detail-modal",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"github.io",
		"/next-portfolio",
		"admin.js",
		"sheet-about",
	} {
		if strings.Contains(html, unwanted) {
			t.Errorf("public HTML must not contain %q", unwanted)
		}
	}

	// 2. Conditional GET with If-None-Match -> 304
	req304 := httptest.NewRequest(http.MethodGet, "/", nil)
	req304.Header.Set("If-None-Match", etag)
	rec304 := httptest.NewRecorder()
	h.ServeHTTP(rec304, req304)
	if rec304.Code != http.StatusNotModified {
		t.Fatalf("conditional GET status = %d, want 304", rec304.Code)
	}

	// 3. SEO & Asset routes
	routes := []string{
		"/robots.txt",
		"/sitemap.xml",
		"/manifest.webmanifest",
		"/favicon.ico",
		"/apple-touch-icon.png",
		"/healthz",
		"/static/images/profile.webp",
		"/static/images/og-image.jpg",
		"/static/files/resume.pdf",
	}
	for _, path := range routes {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
		}
	}

	// 4. Unknown path -> 404
	req404 := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec404 := httptest.NewRecorder()
	h.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist = %d, want 404", rec404.Code)
	}
}

func TestAuthCSRFCRUDAndUploads(t *testing.T) {
	h, _ := setupTestServer(t)

	// Unauthenticated API must return 401
	unauthReq := httptest.NewRequest(http.MethodPut, "/api/about-me", strings.NewReader(`{"bio":"hack"}`))
	unauthRec := httptest.NewRecorder()
	h.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated PUT /api/about-me = %d, want 401", unauthRec.Code)
	}

	// Wrong login -> 401
	badLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"neddy.prasetio@gmail.com","password":"wrong"}`))
	badLogin.RemoteAddr = "198.51.100.5:1234"
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, badLogin)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, want 401", badRec.Code)
	}

	// Valid login
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"neddy.prasetio@gmail.com","password":"admin-secret-123"}`))
	loginReq.RemoteAddr = "198.51.100.5:1234"
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("valid login = %d, body=%s", loginRec.Code, loginRec.Body.String())
	}

	cookies := loginRec.Result().Cookies()
	var csrf string
	for _, c := range cookies {
		if c.Name == auth.CSRFCookieName {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("missing CSRF cookie on login")
	}

	addAuth := func(r *http.Request, includeCSRF bool) {
		for _, c := range cookies {
			r.AddCookie(c)
		}
		if includeCSRF {
			r.Header.Set("X-CSRF-Token", csrf)
		}
	}

	// Authenticated GET / renders admin drawers & no-store
	adminHome := httptest.NewRequest(http.MethodGet, "/", nil)
	addAuth(adminHome, false)
	adminHomeRec := httptest.NewRecorder()
	h.ServeHTTP(adminHomeRec, adminHome)
	if adminHomeRec.Code != http.StatusOK || !strings.Contains(adminHomeRec.Body.String(), "admin.js") {
		t.Fatalf("admin home missing admin.js or bad status=%d", adminHomeRec.Code)
	}
	if !strings.Contains(adminHomeRec.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("admin home must not be publicly cached: %q", adminHomeRec.Header().Get("Cache-Control"))
	}

	// Missing CSRF rejected (403)
	noCSRFReq := httptest.NewRequest(http.MethodPut, "/api/about-me", strings.NewReader(`{"headline":"Go Engineer"}`))
	addAuth(noCSRFReq, false)
	noCSRFRec := httptest.NewRecorder()
	h.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF = %d, want 403", noCSRFRec.Code)
	}

	// Valid PUT /api/about-me updates DB & invalidates cache
	updReq := httptest.NewRequest(http.MethodPut, "/api/about-me", strings.NewReader(`{"headline":"Full-Stack & Go Engineer"}`))
	addAuth(updReq, true)
	updRec := httptest.NewRecorder()
	h.ServeHTTP(updRec, updReq)
	if updRec.Code != http.StatusOK {
		t.Fatalf("PUT /api/about-me = %d, body=%s", updRec.Code, updRec.Body.String())
	}

	// Verify public cache invalidated & shows new headline
	pubReq := httptest.NewRequest(http.MethodGet, "/", nil)
	pubRec := httptest.NewRecorder()
	h.ServeHTTP(pubRec, pubReq)
	if !strings.Contains(pubRec.Body.String(), "Full-Stack &amp; Go Engineer") {
		t.Fatal("expected updated headline in public HTML after cache invalidation")
	}

	// Unsafe javascript: URL rejected on project creation
	badProj := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"title":"Evil","project_link":"javascript:alert(1)"}`))
	addAuth(badProj, true)
	badProjRec := httptest.NewRecorder()
	h.ServeHTTP(badProjRec, badProj)
	if badProjRec.Code != http.StatusBadRequest {
		t.Fatalf("unsafe project_link = %d, want 400", badProjRec.Code)
	}

	// Valid POST /api/projects with showcase metadata persists and renders
	newProj := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{
		"title": "Telemetry Edge",
		"featured": true,
		"category": "Production",
		"role": "Lead Engineer",
		"year": "2026",
		"highlights": ["Handled 10k req/s", "Zero-downtime rollout"],
		"technologies": ["Go", "SQLite"],
		"display_order": 99
	}`))
	addAuth(newProj, true)
	newProjRec := httptest.NewRecorder()
	h.ServeHTTP(newProjRec, newProj)
	if newProjRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/projects = %d, body=%s", newProjRec.Code, newProjRec.Body.String())
	}

	afterProjReq := httptest.NewRequest(http.MethodGet, "/", nil)
	afterProjRec := httptest.NewRecorder()
	h.ServeHTTP(afterProjRec, afterProjReq)
	afterBody := afterProjRec.Body.String()
	for _, want := range []string{"Telemetry Edge", "Lead Engineer", "Handled 10k req/s", "2026"} {
		if !strings.Contains(afterBody, want) {
			t.Fatalf("expected %q in rendered HTML after POST /api/projects", want)
		}
	}

	// Valid PNG upload + serve
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
	upRes := uploadMultipart(t, h, addAuth, "project", "thumb.png", pngBytes)
	if upRes.Code != http.StatusOK {
		t.Fatalf("valid PNG upload = %d, body=%s", upRes.Code, upRes.Body.String())
	}
	var upJSON struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(upRes.Body.Bytes(), &upJSON); err != nil || !strings.HasPrefix(upJSON.URL, "/uploads/project/") {
		t.Fatalf("unexpected upload response: %s", upRes.Body.String())
	}

	// Serve uploaded file back
	serveReq := httptest.NewRequest(http.MethodGet, upJSON.URL, nil)
	serveRec := httptest.NewRecorder()
	h.ServeHTTP(serveRec, serveReq)
	if serveRec.Code != http.StatusOK || serveRec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("GET %s = %d ct=%q", upJSON.URL, serveRec.Code, serveRec.Header().Get("Content-Type"))
	}

	// Spoofed upload rejected
	spoofRes := uploadMultipart(t, h, addAuth, "project", "evil.png", []byte("<script>alert(1)</script>"))
	if spoofRes.Code != http.StatusBadRequest {
		t.Fatalf("spoofed PNG upload = %d, want 400", spoofRes.Code)
	}

	// Logout clears cookies
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	addAuth(logoutReq, true)
	logoutRec := httptest.NewRecorder()
	h.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout = %d, want 200", logoutRec.Code)
	}
}

func uploadMultipart(t *testing.T, h http.Handler, addAuth func(*http.Request, bool), fileType, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("type", fileType)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write(content)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	addAuth(req, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
