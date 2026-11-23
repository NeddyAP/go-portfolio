package cache

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// PageCache stores pre-rendered anonymous homepage bytes and its pre-gzipped variant
// so public requests execute zero DB queries, zero template parsing, and zero per-request compression.
type PageCache struct {
	mu           sync.RWMutex
	raw          []byte
	gz           []byte
	etag         string
	lastModified time.Time
	valid        bool
}

// New creates an empty PageCache.
func New() *PageCache {
	return &PageCache{}
}

// Get returns the cached uncompressed and gzipped HTML, ETag, and LastModified timestamp.
func (c *PageCache) Get() (raw, gz []byte, etag string, mod time.Time, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.valid {
		return nil, nil, "", time.Time{}, false
	}
	return c.raw, c.gz, c.etag, c.lastModified, true
}

// Set caches the rendered HTML page and generates its gzipped counterpart and ETag.
func (c *PageCache) Set(html []byte, mod time.Time) {
	if mod.IsZero() {
		mod = time.Now().UTC()
	}

	rawCopy := make([]byte, len(html))
	copy(rawCopy, html)

	var gzBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	var gzBytes []byte
	if err == nil {
		if _, werr := gw.Write(rawCopy); werr == nil {
			if cerr := gw.Close(); cerr == nil {
				gzBytes = gzBuf.Bytes()
			}
		}
	}

	sum := sha256.Sum256(rawCopy)
	etag := `W/"` + hex.EncodeToString(sum[:8]) + `"`

	c.mu.Lock()
	c.raw = rawCopy
	c.gz = gzBytes
	c.etag = etag
	c.lastModified = mod.UTC().Truncate(time.Second)
	c.valid = true
	c.mu.Unlock()
}

// Invalidate clears the cached page so the next public request rebuilds it.
func (c *PageCache) Invalidate() {
	c.mu.Lock()
	c.raw = nil
	c.gz = nil
	c.etag = ""
	c.lastModified = time.Time{}
	c.valid = false
	c.mu.Unlock()
}
