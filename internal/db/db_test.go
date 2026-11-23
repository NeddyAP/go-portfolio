package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/models"
	_ "modernc.org/sqlite"
)

func TestSeedIdempotentAndCRUD(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	if err := store.SeedIfEmpty(ctx); err != nil {
		t.Fatalf("SeedIfEmpty #1: %v", err)
	}
	if err := store.SeedIfEmpty(ctx); err != nil {
		t.Fatalf("SeedIfEmpty #2: %v", err)
	}

	portfolio, err := store.GetPortfolio(ctx)
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}

	if portfolio.About.FullName != "Neddy Avgha Prasetio" {
		t.Fatalf("unexpected full_name: %q", portfolio.About.FullName)
	}
	if portfolio.About.ProfileImageURL != "/static/images/profile.webp" {
		t.Fatalf("unexpected profile_image_url: %q", portfolio.About.ProfileImageURL)
	}
	if len(portfolio.Experiences) != 5 {
		t.Fatalf("expected 5 experiences, got %d", len(portfolio.Experiences))
	}
	if portfolio.Experiences[3].CompanyName != "PT. Helios Informatika Nusantara" {
		t.Fatalf("expected fixed company typo, got %q", portfolio.Experiences[3].CompanyName)
	}
	if len(portfolio.Certificates) != 5 {
		t.Fatalf("expected 5 certificates, got %d", len(portfolio.Certificates))
	}
	if portfolio.Certificates[0].Title != "DICAS & DICSS 9th Djuanda" {
		t.Fatalf("expected fixed cert title, got %q", portfolio.Certificates[0].Title)
	}
	if len(portfolio.Projects) != 7 {
		t.Fatalf("expected 7 projects, got %d", len(portfolio.Projects))
	}
	featuredProj := portfolio.Projects[0]
	if featuredProj.Title != "E-LGCertified" || !featuredProj.Featured || featuredProj.Category != "Production" || len(featuredProj.Highlights) == 0 {
		t.Fatalf("unexpected seeded featured project: %+v", featuredProj)
	}

	// Partial update on skillset must preserve bio and tools
	origBio := portfolio.About.Bio
	origToolsCount := len(portfolio.About.Tools)
	newSkills := []models.IconItem{{Name: "Go", IconName: "SiGo", Order: 0}}
	updatedAbout, err := store.UpdateAboutMe(ctx, models.AboutMeUpdate{
		Skillset: &newSkills,
	})
	if err != nil {
		t.Fatalf("UpdateAboutMe: %v", err)
	}
	if updatedAbout.Bio != origBio || len(updatedAbout.Tools) != origToolsCount {
		t.Fatalf("partial UpdateAboutMe overwrote unrelated fields")
	}
	if len(updatedAbout.Skillset) != 1 || updatedAbout.Skillset[0].Name != "Go" {
		t.Fatalf("skillset not updated: %+v", updatedAbout.Skillset)
	}

	// Experience CRUD
	exp, err := store.CreateExperience(ctx, models.Experience{
		JobTitle:     "Go Engineer",
		CompanyName:  "Test Corp",
		Location:     "Jakarta",
		StartDate:    "2026-01-01",
		DisplayOrder: 0,
	})
	if err != nil {
		t.Fatalf("CreateExperience: %v", err)
	}
	exp.Location = "Bandung"
	updatedExp, err := store.UpdateExperience(ctx, exp.ID, *exp)
	if err != nil || updatedExp.Location != "Bandung" {
		t.Fatalf("UpdateExperience: %v, loc=%q", err, updatedExp.Location)
	}
	if err := store.DeleteExperience(ctx, exp.ID); err != nil {
		t.Fatalf("DeleteExperience: %v", err)
	}

	// Certificate CRUD
	cert, err := store.CreateCertificate(ctx, models.Certificate{
		Title:               "CKA",
		IssuingOrganization: "CNCF",
		IssueDate:           "2026-05-01",
	})
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	if err := store.DeleteCertificate(ctx, cert.ID); err != nil {
		t.Fatalf("DeleteCertificate: %v", err)
	}

	// Project CRUD
	proj, err := store.CreateProject(ctx, models.Project{
		Title:        "Go Portfolio",
		Featured:     true,
		Category:     "Web App",
		Role:         "Solo Engineer",
		Year:         "2026",
		Highlights:   []string{"Single static binary", "Embedded templates & assets"},
		Technologies: []string{"Go", "SQLite"},
		DisplayOrder: 0,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !proj.Featured || proj.Category != "Web App" || proj.Role != "Solo Engineer" || proj.Year != "2026" || len(proj.Highlights) != 2 {
		t.Fatalf("CreateProject metadata mismatch: %+v", proj)
	}

	updatedProj, err := store.UpdateProject(ctx, proj.ID, models.Project{
		Title:        "Go Portfolio v2",
		Featured:     false,
		Category:     "Open Source",
		Role:         "Author",
		Year:         "2026",
		Highlights:   []string{"Zero runtime dependencies"},
		Technologies: []string{"Go", "SQLite", "HTML"},
		DisplayOrder: 1,
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updatedProj.Featured || updatedProj.Category != "Open Source" || updatedProj.Role != "Author" || len(updatedProj.Highlights) != 1 {
		t.Fatalf("UpdateProject metadata mismatch: %+v", updatedProj)
	}

	if err := store.DeleteProject(ctx, proj.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
}

func TestMigrateLegacyProjectsTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "portfolio.db")

	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = legacyDB.Exec(`
		CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			image_url TEXT NOT NULL DEFAULT '',
			project_link TEXT NOT NULL DEFAULT '',
			source_code_link TEXT NOT NULL DEFAULT '',
			technologies_json TEXT NOT NULL DEFAULT '[]',
			display_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO projects (id, title, description, image_url, project_link, source_code_link, technologies_json, display_order, created_at, updated_at)
		VALUES ('7f49cfa3-8de7-4cd3-8db4-69492e387ec4', 'E-LGCertified', 'Legacy description', '/img.jpg', 'https://e-lgcertified.com/', '', '["Laravel"]', 0, '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z');
	`)
	if err != nil {
		legacyDB.Close()
		t.Fatalf("create legacy projects table: %v", err)
	}
	legacyDB.Close()

	store, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open on legacy DB: %v", err)
	}
	defer store.Close()

	projects, err := store.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects after migration: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 migrated project, got %d", len(projects))
	}
	p := projects[0]
	if !p.Featured || p.Category != "Production" || p.Role == "" || p.Year == "" || len(p.Highlights) == 0 {
		t.Fatalf("expected legacy seeded project to be backfilled, got %+v", p)
	}
}
