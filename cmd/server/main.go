package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/NeddyAP/go-portfolio/internal/auth"
	"github.com/NeddyAP/go-portfolio/internal/cache"
	"github.com/NeddyAP/go-portfolio/internal/config"
	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/github"
	"github.com/NeddyAP/go-portfolio/internal/handlers"
	"github.com/NeddyAP/go-portfolio/web"
)

const version = "1.0.0"

func main() {
	hashPwFlag := flag.Bool("hash-password", false, "Generate a bcrypt hash for ADMIN_PASSWORD_HASH from stdin or first positional arg")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("go-portfolio", version)
		return
	}

	if *hashPwFlag {
		var pw string
		if flag.NArg() > 0 {
			pw = flag.Arg(0)
		} else {
			fi, err := os.Stdin.Stat()
			if err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
				fmt.Fprint(os.Stderr, "Enter password (min 8 chars): ")
			}
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil && len(line) == 0 {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			pw = strings.TrimRight(line, "\r\n")
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(hash)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "uploads"), 0o755); err != nil {
		slog.Error("failed to create uploads directory", "err", err)
		os.Exit(1)
	}

	store, err := db.Open(cfg.DataDir)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.SeedIfEmpty(context.Background()); err != nil {
		slog.Error("failed to seed database", "err", err)
		os.Exit(1)
	}

	authMgr := auth.NewManager(cfg.AdminEmail, cfg.AdminPasswordHash, cfg.SessionSecret, cfg.TrustProxy)
	if !authMgr.IsConfigured() {
		slog.Warn("ADMIN_PASSWORD_HASH not set; admin login is disabled. Generate one with: ./portfolio -hash-password")
	}

	pageCache := cache.New()
	ghService := github.NewService(cfg.GitHubAPIURL, pageCache.Invalidate)

	if about, err := store.GetAboutMe(context.Background()); err == nil {
		if username := github.ParseUsername(about.GitHubURL); username != "" {
			ghService.WarmAsync(username)
		}
	}

	srv, err := handlers.NewServer(cfg, store, authMgr, pageCache, ghService, web.TemplatesFS, web.StaticFS)
	if err != nil {
		slog.Error("failed to initialize server", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              cfg.Address(),
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("portfolio server listening",
			"addr", cfg.Address(),
			"site_url", cfg.SiteURL,
			"data_dir", cfg.DataDir,
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("shutting down gracefully", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown error", "err", err)
		}
	case err := <-errCh:
		if err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}
}
