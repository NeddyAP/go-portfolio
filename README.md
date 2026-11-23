# Neddy Avgha Prasetio — Go Portfolio (`neddyap.tech`)

Single-binary Go (`net/http` + `html/template` + pure-Go SQLite) recreation of [NeddyAP/next-portfolio](https://github.com/NeddyAP/next-portfolio), optimized for low-memory NAT VPS deployment behind Cloudflare Tunnel and `systemd`.

## Key Improvements Over the Original Next.js + Supabase App

- **Zero External Runtime Dependencies**: No Node.js runtime, no Next.js cold memory overhead, and no Supabase dependency. Pure-Go SQLite (`modernc.org/sqlite` with WAL) and local disk uploads (`DATA_DIR/uploads`) run inside a single static binary.
- **Ultra-Low Memory & Fast Responses**: Public visitors are served from a pre-rendered, pre-gzipped in-memory page cache (`ETag` + `304 Not Modified`), skipping DB reads and template rendering per request.
- **Asset Optimization**: All live images and screenshots from Supabase/GitHub are self-hosted and compressed to WebP (~260 KB total for all 7 project thumbnails vs. ~3.4 MB previously), plus a dedicated `1200x630` OpenGraph card (`og-image.jpg`) and `apple-touch-icon.png`.
- **Fixed Bugs & UX Polish**:
  - Fixed SEO canonical URLs, `sitemap.xml`, `robots.txt`, `manifest.webmanifest`, `favicon.ico`, and JSON-LD (`neddyap.tech` instead of `neddyap.github.io/next-portfolio`).
  - Fixed theme toggle when defaulting to system preference, eliminated theme FOUC with `/static/js/theme-init.js`, and made the mobile drawer theme-aware and auto-closing.
  - Added `#certificates` to the navigation bar, rendered `location` in experience cards, unified nested experience card borders, and added one-click email copy in Contact.
  - Fixed overlapping hover-only skill/tool labels so every badge shows its icon and label cleanly on both mobile and desktop.
  - Replaced `react-github-calendar` with a non-blocking, cached, server-rendered SVG contribution graph (`stale-while-revalidate`).
  - Kept lazy PDF previews with a non-overlapping toolbar (`Open PDF`, `Fullscreen`, `Hide Preview`).
- **Hardened Security**: Bcrypt password hashing, HMAC-SHA256 `HttpOnly` session cookies, double-submit CSRF verification, per-IP login rate limiting, strict magic-byte MIME upload validation, and strict CSP (`script-src 'self'`).

## Local Development

```bash
cp deploy/.env.example .env
# Generate an admin password hash and paste into ADMIN_PASSWORD_HASH in .env
go run ./cmd/server -hash-password
# Add SESSION_SECRET (e.g. openssl rand -hex 32) to .env, then start:
make run
```

Visit `http://127.0.0.1:8080`. Click **Neddy** in the footer copyright line to open the Admin Login modal.

## Building & Deploying on a NAT VPS (Cloudflare Tunnel + systemd)

1. Build on the VPS (or cross-compile locally with `make build-linux-amd64` / `make build-linux-arm64`):
   ```bash
   make build
   ```
2. Install binary & data directory:
   ```bash
   sudo mkdir -p /opt/neddy-portfolio/data/uploads
   sudo cp bin/portfolio /opt/neddy-portfolio/portfolio
   sudo chown -R www-data:www-data /opt/neddy-portfolio
   ```
3. Configure `/etc/neddy-portfolio.env` from `deploy/.env.example`:
   ```bash
   sudo cp deploy/.env.example /etc/neddy-portfolio.env
   # Generate password hash & session secret:
   /opt/neddy-portfolio/portfolio -hash-password
   openssl rand -hex 32
   ```
4. Enable the `systemd` service (`deploy/portfolio.service`):
   ```bash
   sudo cp deploy/portfolio.service /etc/systemd/system/neddy-portfolio.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now neddy-portfolio
   ```
5. Point Cloudflare Tunnel (`deploy/cloudflared-config.example.yml`) to `http://127.0.0.1:8080`.
6. Backup SQLite + uploads anytime by backing up `/opt/neddy-portfolio/data/` (or `sqlite3 portfolio.db ".backup backup.db"`).
