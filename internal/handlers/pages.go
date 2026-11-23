package handlers

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NeddyAP/go-portfolio/internal/github"
	"github.com/NeddyAP/go-portfolio/internal/icons"
	"github.com/NeddyAP/go-portfolio/internal/models"
)

// ProjectFilter describes a filter pill displayed above the portfolio showcase.
type ProjectFilter struct {
	Key   string
	Label string
	Count int
}

type pageViewData struct {
	SiteURL          string
	Title            string
	Description      string
	Year             int
	IsAdmin          bool
	AdminEmail       string
	CSRFToken        string
	Portfolio        *models.PortfolioData
	FeaturedProjects []models.Project
	GridProjects     []models.Project
	ProjectFilters   []ProjectFilter
	Calendar         *github.CalendarData
	AvailableIcons   []icons.IconOption
}

func (s *Server) buildViewData(r *http.Request, isAdmin bool, adminEmail, csrf string) (*pageViewData, error) {
	portfolio, err := s.store.GetPortfolio(r.Context())
	if err != nil {
		return nil, err
	}

	var cal *github.CalendarData
	if username := github.ParseUsername(portfolio.About.GitHubURL); username != "" {
		cal = s.github.GetCached(username)
	}

	desc := "Portfolio of " + portfolio.About.FullName + ", a " + portfolio.About.Headline + " from Indonesia specializing in Laravel, PHP, React, JavaScript, Go, and modern web systems."
	var availIcons []icons.IconOption
	if isAdmin {
		availIcons = icons.AvailableIcons()
	}

	featured, grid, filters := buildProjectShowcase(portfolio.Projects)

	return &pageViewData{
		SiteURL:          s.cfg.SiteURL,
		Title:            portfolio.About.FullName + " - " + portfolio.About.Headline + " Portfolio",
		Description:      desc,
		Year:             time.Now().Year(),
		IsAdmin:          isAdmin,
		AdminEmail:       adminEmail,
		CSRFToken:        csrf,
		Portfolio:        portfolio,
		FeaturedProjects: featured,
		GridProjects:     grid,
		ProjectFilters:   filters,
		Calendar:         cal,
		AvailableIcons:   availIcons,
	}, nil
}

func buildProjectShowcase(projects []models.Project) (featured []models.Project, grid []models.Project, filters []ProjectFilter) {
	liveCount := 0
	ossCount := 0
	type techStat struct {
		label string
		count int
		first int
	}
	techMap := make(map[string]*techStat)
	orderIdx := 0

	for _, p := range projects {
		if p.Featured {
			featured = append(featured, p)
		} else {
			grid = append(grid, p)
		}
		if strings.TrimSpace(p.ProjectLink) != "" {
			liveCount++
		}
		if strings.TrimSpace(p.SourceCodeLink) != "" {
			ossCount++
		}
		seenInProj := make(map[string]bool)
		for _, t := range p.Technologies {
			label := strings.TrimSpace(t)
			key := strings.ToLower(label)
			if key == "" || seenInProj[key] {
				continue
			}
			seenInProj[key] = true
			if st, ok := techMap[key]; ok {
				st.count++
			} else {
				techMap[key] = &techStat{label: label, count: 1, first: orderIdx}
				orderIdx++
			}
		}
	}

	filters = append(filters, ProjectFilter{Key: "all", Label: "All", Count: len(projects)})
	if liveCount > 0 {
		filters = append(filters, ProjectFilter{Key: "live", Label: "Live", Count: liveCount})
	}
	if ossCount > 0 {
		filters = append(filters, ProjectFilter{Key: "opensource", Label: "Open Source", Count: ossCount})
	}

	stats := make([]*techStat, 0, len(techMap))
	for _, st := range techMap {
		stats = append(stats, st)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].count != stats[j].count {
			return stats[i].count > stats[j].count
		}
		return stats[i].first < stats[j].first
	})

	// Prioritize distinctive stack filters (skipping ubiquitous MySQL/PHP if other stacks exist, or include top 5 overall cleanly)
	limit := 5
	if len(stats) < limit {
		limit = len(stats)
	}
	for _, st := range stats[:limit] {
		filters = append(filters, ProjectFilter{
			Key:   "tech:" + strings.ToLower(st.label),
			Label: st.label,
			Count: st.count,
		})
	}
	return featured, grid, filters
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.render404(w, r)
		return
	}

	if sess, ok := s.auth.GetSession(r); ok {
		data, err := s.buildViewData(r, true, sess.Email, sess.CSRF)
		if err != nil {
			slog.Error("load admin portfolio", "err", err)
			http.Error(w, "Failed to load portfolio", http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		if err := s.indexTmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("render admin template", "err", err)
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store, max-age=0")
		w.Header().Set("Vary", "Cookie")
		_, _ = w.Write(buf.Bytes())
		return
	}

	raw, gz, etag, mod, ok := s.cache.Get()
	if !ok {
		data, err := s.buildViewData(r, false, "", "")
		if err != nil {
			slog.Error("load public portfolio", "err", err)
			http.Error(w, "Failed to load portfolio", http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		if err := s.indexTmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("render public template", "err", err)
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}
		lastMod := time.Now().UTC()
		if parsed, err := time.Parse(time.RFC3339, data.Portfolio.About.UpdatedAt); err == nil {
			lastMod = parsed
		}
		s.cache.Set(buf.Bytes(), lastMod)
		raw, gz, etag, mod, _ = s.cache.Get()
	}

	if inm := strings.TrimSpace(r.Header.Get("If-None-Match")); inm != "" && inm == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("ETag", etag)
	if !mod.IsZero() {
		w.Header().Set("Last-Modified", mod.UTC().Format(http.TimeFormat))
	}
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	w.Header().Set("Vary", "Accept-Encoding")

	if r.Method != http.MethodHead && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && len(gz) > 0 {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gz)
		return
	}

	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(raw)
	}
}

func (s *Server) render404(w http.ResponseWriter, r *http.Request) {
	about, err := s.store.GetAboutMe(r.Context())
	if err != nil {
		about = &models.AboutMe{FullName: "Neddy Avgha Prasetio", Headline: "Full-Stack Software Engineer"}
	}
	data := pageViewData{
		SiteURL:     s.cfg.SiteURL,
		Title:       "404 - Page Not Found | " + about.FullName,
		Description: "The page you are looking for does not exist.",
		Year:        time.Now().Year(),
		Portfolio:   &models.PortfolioData{About: *about},
	}
	var buf bytes.Buffer
	if err := s.notFoundTmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	s.serveEmbeddedFile(w, r, "images/favicon.ico", "image/x-icon", "public, max-age=604800")
}

func (s *Server) handleAppleTouchIcon(w http.ResponseWriter, r *http.Request) {
	s.serveEmbeddedFile(w, r, "images/apple-touch-icon.png", "image/png", "public, max-age=604800")
}

func (s *Server) serveEmbeddedFile(w http.ResponseWriter, r *http.Request, relPath, contentType, cacheControl string) {
	f, err := s.staticFS.Open(relPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
		rs = bytes.NewReader(data)
	}

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(path.Clean("/"+r.PathValue("path")), "/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}

	f, err := s.staticFS.Open(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
		rs = bytes.NewReader(data)
	}

	if ct := mimeByExt(rel); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
}

func (s *Server) handleUploadServe(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	filename := r.PathValue("filename")

	if !isAllowedUploadCategory(category) {
		http.NotFound(w, r)
		return
	}
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(s.cfg.DataDir, "uploads", category, filename)
	f, err := os.Open(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	ct := mimeByExt(filename)
	if ct == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filename, info.ModTime(), f)
}

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	body := "User-agent: *\nAllow: /\nDisallow: /api/\n\nSitemap: " + s.cfg.SiteURL + "/sitemap.xml\n"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(body))
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	about, err := s.store.GetAboutMe(r.Context())
	lastMod := time.Now().UTC().Format(time.RFC3339)
	if err == nil && about.UpdatedAt != "" {
		lastMod = about.UpdatedAt
	}

	type sitemapURL struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod"`
		ChangeFreq string `xml:"changefreq"`
		Priority   string `xml:"priority"`
	}
	type urlSet struct {
		XMLName xml.Name     `xml:"urlset"`
		Xmlns   string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}

	doc := urlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs: []sitemapURL{
			{
				Loc:        s.cfg.SiteURL + "/",
				LastMod:    lastMod,
				ChangeFreq: "monthly",
				Priority:   "1.0",
			},
		},
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(doc)
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	about, err := s.store.GetAboutMe(r.Context())
	name := "Neddy Avgha Prasetio - Portfolio"
	if err == nil && about.FullName != "" {
		name = about.FullName + " - Portfolio"
	}

	manifest := map[string]any{
		"name":             name,
		"short_name":       "Neddy AP",
		"description":      "Portfolio of Neddy Avgha Prasetio, a Full-Stack Software Engineer from Indonesia.",
		"start_url":        "/",
		"display":          "standalone",
		"background_color": "#0a0a1f",
		"theme_color":      "#0a0a1f",
		"icons": []map[string]string{
			{
				"src":   "/apple-touch-icon.png",
				"sizes": "180x180",
				"type":  "image/png",
			},
			{
				"src":   "/favicon.ico",
				"sizes": "any",
				"type":  "image/x-icon",
			},
		},
	}

	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = json.NewEncoder(w).Encode(manifest)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func mimeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".webp":
		return "image/webp"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".svg":
		return "image/svg+xml"
	default:
		return ""
	}
}

func isAllowedUploadCategory(cat string) bool {
	switch cat {
	case "profile", "resume", "project", "certificate":
		return true
	default:
		return false
	}
}
