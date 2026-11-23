package db_test

import (
	"context"
	"testing"

	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/models"
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
		Technologies: []string{"Go", "SQLite"},
		DisplayOrder: 0,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := store.DeleteProject(ctx, proj.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
}
