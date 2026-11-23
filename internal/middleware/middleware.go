package middleware

import (
	"compress/gzip"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Recover guards handlers from panics and returns a clean 500 error.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "path", r.URL.Path, "stack", string(debug.Stack()))
				if strings.HasPrefix(r.URL.Path, "/api/") {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"Internal server error"}`))
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds strict security headers including a zero-unsafe-inline-script CSP.
func SecurityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; frame-src 'self'; object-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

var gzipPool = sync.Pool{
	New: func() any {
		gw, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return gw
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gw          *gzip.Writer
	wroteHeader bool
	decided     bool
	compressed  bool
	buf         []byte
	statusCode  int
}

func (g *gzipResponseWriter) Header() http.Header {
	return g.ResponseWriter.Header()
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.statusCode = code
	g.wroteHeader = true

	if g.Header().Get("Content-Encoding") != "" || code == http.StatusNoContent || code == http.StatusNotModified {
		g.decided = true
		g.ResponseWriter.WriteHeader(code)
		return
	}
}

func (g *gzipResponseWriter) Write(p []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.decided {
		if g.compressed {
			return g.gw.Write(p)
		}
		return g.ResponseWriter.Write(p)
	}

	g.buf = append(g.buf, p...)
	if len(g.buf) < 512 {
		return len(p), nil
	}
	if err := g.flushDecision(true); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (g *gzipResponseWriter) flushDecision(allowCompress bool) error {
	g.decided = true
	ct := g.Header().Get("Content-Type")
	if ct == "" && len(g.buf) > 0 {
		ct = http.DetectContentType(g.buf)
		g.Header().Set("Content-Type", ct)
	}
	if allowCompress && isCompressibleType(ct) && len(g.buf) >= 512 {
		g.compressed = true
		g.Header().Del("Content-Length")
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Add("Vary", "Accept-Encoding")
		g.ResponseWriter.WriteHeader(g.statusCode)
		gw := gzipPool.Get().(*gzip.Writer)
		gw.Reset(g.ResponseWriter)
		g.gw = gw
		_, err := g.gw.Write(g.buf)
		g.buf = nil
		return err
	}

	g.ResponseWriter.WriteHeader(g.statusCode)
	if len(g.buf) > 0 {
		_, err := g.ResponseWriter.Write(g.buf)
		g.buf = nil
		return err
	}
	return nil
}

func (g *gzipResponseWriter) finish() {
	if !g.wroteHeader {
		return
	}
	if g.compressed && g.gw != nil {
		_ = g.gw.Close()
		g.gw.Reset(io.Discard)
		gzipPool.Put(g.gw)
		g.gw = nil
		return
	}
	if !g.decided {
		_ = g.flushDecision(false)
	}
}

func isCompressibleType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "application/javascript") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/manifest+json") ||
		strings.Contains(ct, "image/svg+xml")
}

// Gzip transparently compresses compressible responses >= 512 bytes when supported by the client.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		grw := &gzipResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		defer grw.finish()
		next.ServeHTTP(grw, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger logs concise structured request telemetry.
func Logger(trustProxy bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if strings.HasPrefix(r.URL.Path, "/static/") && rec.status < 400 {
			return
		}
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", RealIP(r, trustProxy),
		)
	})
}

// RealIP resolves the client IP, honoring Cloudflare/proxy headers when trustProxy is true.
func RealIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
			return cfIP
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			if first, _, ok := strings.Cut(xff, ","); ok {
				return strings.TrimSpace(first)
			}
			return xff
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
