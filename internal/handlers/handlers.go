package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/NeddyAP/go-portfolio/internal/auth"
	"github.com/NeddyAP/go-portfolio/internal/cache"
	"github.com/NeddyAP/go-portfolio/internal/config"
	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/github"
	"github.com/NeddyAP/go-portfolio/internal/icons"
	"github.com/NeddyAP/go-portfolio/internal/middleware"
	"github.com/NeddyAP/go-portfolio/internal/models"
)

// Server wires all HTTP handlers, templates, caching, DB, and auth.
type Server struct {
	cfg          *config.Config
	store        *db.Store
	auth         *auth.Manager
	cache        *cache.PageCache
	github       *github.Service
	indexTmpl    *template.Template
	notFoundTmpl *template.Template
	staticFS     fs.FS
	bootHash     string
}

// NewServer constructs a Server from embedded template & static filesystems.
func NewServer(
	cfg *config.Config,
	store *db.Store,
	authMgr *auth.Manager,
	pageCache *cache.PageCache,
	ghService *github.Service,
	templatesFS fs.FS,
	staticRootFS fs.FS,
) (*Server, error) {
	staticSub, err := fs.Sub(staticRootFS, "static")
	if err != nil {
		return nil, fmt.Errorf("sub static fs: %w", err)
	}

	bootHash := computeAssetsHash(staticSub)
	s := &Server{
		cfg:      cfg,
		store:    store,
		auth:     authMgr,
		cache:    pageCache,
		github:   ghService,
		staticFS: staticSub,
		bootHash: bootHash,
	}

	funcs := s.templateFuncs()
	idxTmpl, err := template.New("base.html").Funcs(funcs).ParseFS(
		templatesFS,
		"templates/base.html",
		"templates/index.html",
		"templates/partials/admin_drawers.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse index templates: %w", err)
	}

	nfTmpl, err := template.New("base.html").Funcs(funcs).ParseFS(
		templatesFS,
		"templates/base.html",
		"templates/404.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse 404 templates: %w", err)
	}

	s.indexTmpl = idxTmpl
	s.notFoundTmpl = nfTmpl
	return s, nil
}

// Routes returns the full HTTP handler chain with security, logging, and gzip middleware.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
	mux.HandleFunc("GET /apple-touch-icon.png", s.handleAppleTouchIcon)
	mux.HandleFunc("GET /robots.txt", s.handleRobots)
	mux.HandleFunc("GET /sitemap.xml", s.handleSitemap)
	mux.HandleFunc("GET /manifest.webmanifest", s.handleManifest)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /static/{path...}", s.handleStatic)
	mux.HandleFunc("GET /uploads/{category}/{filename}", s.handleUploadServe)

	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)

	mux.Handle("PUT /api/about-me", s.requireAdmin(s.handleUpdateAboutMe))
	mux.Handle("POST /api/experiences", s.requireAdmin(s.handleCreateExperience))
	mux.Handle("PUT /api/experiences/{id}", s.requireAdmin(s.handleUpdateExperience))
	mux.Handle("DELETE /api/experiences/{id}", s.requireAdmin(s.handleDeleteExperience))

	mux.Handle("POST /api/certificates", s.requireAdmin(s.handleCreateCertificate))
	mux.Handle("PUT /api/certificates/{id}", s.requireAdmin(s.handleUpdateCertificate))
	mux.Handle("DELETE /api/certificates/{id}", s.requireAdmin(s.handleDeleteCertificate))

	mux.Handle("POST /api/projects", s.requireAdmin(s.handleCreateProject))
	mux.Handle("PUT /api/projects/{id}", s.requireAdmin(s.handleUpdateProject))
	mux.Handle("DELETE /api/projects/{id}", s.requireAdmin(s.handleDeleteProject))

	mux.Handle("POST /api/upload", s.requireAdmin(s.handleUploadPOST))
	mux.Handle("DELETE /api/upload", s.requireAdmin(s.handleUploadDELETE))

	mux.HandleFunc("GET /", s.handleHome)

	var handler http.Handler = mux
	handler = middleware.Gzip(handler)
	handler = middleware.Logger(s.cfg.TrustProxy, handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Recover(handler)
	return handler
}

func (s *Server) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"icon": func(name, className string) template.HTML {
			return icons.Render(name, className)
		},
		"expIcon": func(iconName, jobTitle, className string) template.HTML {
			resolved := icons.ExperienceIcon(iconName, jobTitle)
			return icons.Render(resolved, className)
		},
		"formatDateRange": formatDateRange,
		"formatDate":      formatDate,
		"isPDF":           isPDFURL,
		"assetURL": func(p string) string {
			if strings.HasPrefix(p, "/static/") {
				return p + "?v=" + s.bootHash
			}
			return p
		},
		"absURL": func(p string) string {
			if p == "" {
				return s.cfg.SiteURL
			}
			if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
				return p
			}
			if strings.HasPrefix(p, "/") {
				return s.cfg.SiteURL + p
			}
			return s.cfg.SiteURL + "/" + p
		},
		"safeHref": func(raw string) string {
			if err := ValidateSafeURL(raw, true); err != nil {
				return "#"
			}
			return raw
		},
		"jsonMarshal": func(v any) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("{}")
			}
			return template.JS(b)
		},
		"jsonString": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "{}"
			}
			return string(b)
		},
		"jsonLD": func(data pageViewData) template.HTML {
			return buildJSONLD(s.cfg.SiteURL, data.Portfolio.About)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"eqStr": func(a, b string) bool { return a == b },
		"join": func(items []string, sep string) string {
			return strings.Join(items, sep)
		},
	}
}

func buildJSONLD(siteURL string, about models.AboutMe) template.HTML {
	knows := make([]string, 0, len(about.Skillset))
	for _, sk := range about.Skillset {
		if sk.Name != "" {
			knows = append(knows, sk.Name)
		}
	}
	if len(knows) == 0 {
		knows = []string{"PHP", "Laravel", "JavaScript", "React", "Go", "MySQL", "PostgreSQL", "Docker"}
	}

	var sameAs []string
	if about.GitHubURL != "" {
		sameAs = append(sameAs, about.GitHubURL)
	}
	if about.LinkedInURL != "" {
		sameAs = append(sameAs, about.LinkedInURL)
	}

	imgURL := about.ProfileImageURL
	if strings.HasPrefix(imgURL, "/") {
		imgURL = siteURL + imgURL
	} else if imgURL == "" {
		imgURL = siteURL + "/static/images/profile.webp"
	}

	person := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Person",
		"name":          about.FullName,
		"alternateName": "Neddy AP",
		"url":           siteURL,
		"image":         imgURL,
		"jobTitle":      about.Headline,
		"description":   about.Bio,
		"address": map[string]any{
			"@type":           "PostalAddress",
			"addressLocality": "Bogor",
			"addressCountry":  "ID",
		},
		"sameAs":     sameAs,
		"knowsAbout": knows,
		"alumniOf": map[string]any{
			"@type": "EducationalOrganization",
			"name":  "Djuanda University",
		},
	}

	website := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebSite",
		"name":        about.FullName + " Portfolio",
		"url":         siteURL,
		"description": "Portfolio of " + about.FullName + ", a " + about.Headline + " from Indonesia.",
		"author": map[string]any{
			"@type": "Person",
			"name":  about.FullName,
		},
		"inLanguage": "en-US",
	}

	pJSON, _ := json.Marshal(person)
	wJSON, _ := json.Marshal(website)
	return template.HTML(
		`<script type="application/ld+json">` + string(pJSON) + `</script>` +
			`<script type="application/ld+json">` + string(wJSON) + `</script>`,
	)
}

func computeAssetsHash(staticSub fs.FS) string {
	h := sha256.New()
	for _, p := range []string{"css/styles.css", "js/theme-init.js", "js/app.js", "js/admin.js"} {
		if f, err := staticSub.Open(p); err == nil {
			_, _ = io.Copy(h, f)
			_ = f.Close()
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// ValidateSafeURL ensures user-supplied URLs only use http(s) or clean app-relative paths.
func ValidateSafeURL(raw string, allowRelative bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("URL contains control characters")
		}
	}
	if strings.HasPrefix(raw, "//") {
		return fmt.Errorf("protocol-relative URLs are not allowed")
	}
	if strings.HasPrefix(raw, "/") {
		if !allowRelative {
			return fmt.Errorf("relative URL not allowed here")
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must include a host")
	}
	return nil
}

// ValidateEmail verifies an email address format when non-empty.
func ValidateEmail(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	_, err := mail.ParseAddress(raw)
	return err
}

func formatDateRange(startDate, endDate string) string {
	start := strings.TrimSpace(startDate)
	end := strings.TrimSpace(endDate)
	if start == "" {
		return "Date not specified"
	}
	startFmt := formatMonthYear(start)
	if end == "" || strings.EqualFold(end, "present") {
		return startFmt + " - Present"
	}
	return startFmt + " - " + formatMonthYear(end)
}

func formatMonthYear(raw string) string {
	for _, layout := range []string{"2006-01-02", "2006-01"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("January 2006")
		}
	}
	return raw
}

func formatDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.Format("January 2, 2006")
	}
	if t, err := time.Parse("2006-01", raw); err == nil {
		return t.Format("January 2006")
	}
	return raw
}

func isPDFURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && u.Path != "" {
		return strings.HasSuffix(strings.ToLower(u.Path), ".pdf")
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(raw)), ".pdf")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "message": msg})
}

func (s *Server) sameOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Allow non-browser / curl / same-origin requests without Origin unless Sec-Fetch-Site says cross-site.
		fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
		return fetchSite == "" || fetchSite == "same-origin" || fetchSite == "none"
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	reqHost := r.Host
	if s.cfg.TrustProxy {
		if fh := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fh != "" {
			reqHost = fh
		}
	}
	if strings.EqualFold(u.Host, reqHost) {
		return true
	}
	if siteU, err := url.Parse(s.cfg.SiteURL); err == nil && strings.EqualFold(u.Host, siteU.Host) {
		return true
	}
	return false
}
