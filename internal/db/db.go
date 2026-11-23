package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NeddyAP/go-portfolio/internal/models"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found")

// Store wraps SQLite persistence for all portfolio content.
type Store struct {
	sqlDB *sql.DB
}

// Open opens (or creates) the SQLite database in dataDir, configures WAL/pragmas,
// and runs schema migrations.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "portfolio.db")
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	store := &Store{sqlDB: sqlDB}
	if err := store.Migrate(context.Background()); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return store, nil
}

// Close closes the underlying SQLite connection.
func (s *Store) Close() error {
	return s.sqlDB.Close()
}

// Ping verifies database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.sqlDB.PingContext(ctx)
}

// Migrate ensures all tables and indexes exist.
func (s *Store) Migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS about_me (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	full_name TEXT NOT NULL DEFAULT '',
	headline TEXT NOT NULL DEFAULT '',
	bio TEXT NOT NULL DEFAULT '',
	profile_image_url TEXT NOT NULL DEFAULT '',
	resume_url TEXT NOT NULL DEFAULT '',
	contact_email TEXT NOT NULL DEFAULT '',
	linkedin_url TEXT NOT NULL DEFAULT '',
	github_url TEXT NOT NULL DEFAULT '',
	hobbies_json TEXT NOT NULL DEFAULT '[]',
	skillset_json TEXT NOT NULL DEFAULT '[]',
	tools_json TEXT NOT NULL DEFAULT '[]',
	quote_json TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS experiences (
	id TEXT PRIMARY KEY,
	job_title TEXT NOT NULL,
	company_name TEXT NOT NULL,
	company_logo_url TEXT NOT NULL DEFAULT '',
	location TEXT NOT NULL DEFAULT '',
	start_date TEXT NOT NULL DEFAULT '',
	end_date TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	icon_name TEXT NOT NULL DEFAULT '',
	display_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_experiences_order ON experiences(display_order, start_date DESC);

CREATE TABLE IF NOT EXISTS certificates (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	issuing_organization TEXT NOT NULL,
	issue_date TEXT NOT NULL DEFAULT '',
	credential_url TEXT NOT NULL DEFAULT '',
	certificate_file_url TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_certificates_date ON certificates(issue_date DESC, title ASC);

CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	image_url TEXT NOT NULL DEFAULT '',
	project_link TEXT NOT NULL DEFAULT '',
	source_code_link TEXT NOT NULL DEFAULT '',
	technologies_json TEXT NOT NULL DEFAULT '[]',
	display_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_projects_order ON projects(display_order, created_at DESC);
`
	_, err := s.sqlDB.ExecContext(ctx, schema)
	return err
}

// GetPortfolio fetches all sections in sorted order.
func (s *Store) GetPortfolio(ctx context.Context) (*models.PortfolioData, error) {
	about, err := s.GetAboutMe(ctx)
	if err != nil {
		return nil, err
	}
	exps, err := s.ListExperiences(ctx)
	if err != nil {
		return nil, err
	}
	certs, err := s.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}
	projs, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	return &models.PortfolioData{
		About:        *about,
		Experiences:  exps,
		Certificates: certs,
		Projects:     projs,
	}, nil
}

// GetAboutMe returns the primary profile record (id = 1).
func (s *Store) GetAboutMe(ctx context.Context) (*models.AboutMe, error) {
	const q = `SELECT id, full_name, headline, bio, profile_image_url, resume_url, contact_email, linkedin_url, github_url, hobbies_json, skillset_json, tools_json, quote_json, updated_at FROM about_me WHERE id = 1`
	var a models.AboutMe
	var hobbiesJSON, skillsetJSON, toolsJSON, quoteJSON string
	err := s.sqlDB.QueryRowContext(ctx, q).Scan(
		&a.ID, &a.FullName, &a.Headline, &a.Bio, &a.ProfileImageURL, &a.ResumeURL,
		&a.ContactEmail, &a.LinkedInURL, &a.GitHubURL, &hobbiesJSON, &skillsetJSON,
		&toolsJSON, &quoteJSON, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &models.AboutMe{ID: 1}, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(hobbiesJSON), &a.Hobbies)
	_ = json.Unmarshal([]byte(skillsetJSON), &a.Skillset)
	_ = json.Unmarshal([]byte(toolsJSON), &a.Tools)
	if strings.TrimSpace(quoteJSON) != "" && strings.TrimSpace(quoteJSON) != "null" {
		var qObj models.Quote
		if json.Unmarshal([]byte(quoteJSON), &qObj) == nil && (qObj.Text != "" || qObj.Author != "") {
			a.Quote = &qObj
		}
	}

	sortIconItems(a.Skillset)
	sortIconItems(a.Tools)
	return &a, nil
}

// UpdateAboutMe performs a partial update on about_me row id=1.
func (s *Store) UpdateAboutMe(ctx context.Context, upd models.AboutMeUpdate) (*models.AboutMe, error) {
	current, err := s.GetAboutMe(ctx)
	if err != nil {
		return nil, err
	}

	if upd.FullName != nil {
		current.FullName = strings.TrimSpace(*upd.FullName)
	}
	if upd.Headline != nil {
		current.Headline = strings.TrimSpace(*upd.Headline)
	}
	if upd.Bio != nil {
		current.Bio = strings.TrimSpace(*upd.Bio)
	}
	if upd.ProfileImageURL != nil {
		current.ProfileImageURL = strings.TrimSpace(*upd.ProfileImageURL)
	}
	if upd.ResumeURL != nil {
		current.ResumeURL = strings.TrimSpace(*upd.ResumeURL)
	}
	if upd.ContactEmail != nil {
		current.ContactEmail = strings.TrimSpace(*upd.ContactEmail)
	}
	if upd.LinkedInURL != nil {
		current.LinkedInURL = strings.TrimSpace(*upd.LinkedInURL)
	}
	if upd.GitHubURL != nil {
		current.GitHubURL = strings.TrimSpace(*upd.GitHubURL)
	}
	if upd.Hobbies != nil {
		current.Hobbies = normalizeIconItems(*upd.Hobbies)
	}
	if upd.Skillset != nil {
		current.Skillset = normalizeIconItems(*upd.Skillset)
	}
	if upd.Tools != nil {
		current.Tools = normalizeIconItems(*upd.Tools)
	}
	if upd.ClearQuote {
		current.Quote = nil
	} else if upd.Quote != nil {
		qt := strings.TrimSpace(upd.Quote.Text)
		qa := strings.TrimSpace(upd.Quote.Author)
		if qt == "" && qa == "" {
			current.Quote = nil
		} else {
			current.Quote = &models.Quote{Text: qt, Author: qa}
		}
	}

	current.UpdatedAt = nowISO()

	hobbiesBytes, _ := json.Marshal(current.Hobbies)
	skillsetBytes, _ := json.Marshal(current.Skillset)
	toolsBytes, _ := json.Marshal(current.Tools)
	quoteBytes := []byte("")
	if current.Quote != nil {
		quoteBytes, _ = json.Marshal(current.Quote)
	}

	const q = `
INSERT INTO about_me (id, full_name, headline, bio, profile_image_url, resume_url, contact_email, linkedin_url, github_url, hobbies_json, skillset_json, tools_json, quote_json, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	full_name = excluded.full_name,
	headline = excluded.headline,
	bio = excluded.bio,
	profile_image_url = excluded.profile_image_url,
	resume_url = excluded.resume_url,
	contact_email = excluded.contact_email,
	linkedin_url = excluded.linkedin_url,
	github_url = excluded.github_url,
	hobbies_json = excluded.hobbies_json,
	skillset_json = excluded.skillset_json,
	tools_json = excluded.tools_json,
	quote_json = excluded.quote_json,
	updated_at = excluded.updated_at`

	_, err = s.sqlDB.ExecContext(ctx, q,
		current.FullName, current.Headline, current.Bio, current.ProfileImageURL, current.ResumeURL,
		current.ContactEmail, current.LinkedInURL, current.GitHubURL, string(hobbiesBytes),
		string(skillsetBytes), string(toolsBytes), string(quoteBytes), current.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s.GetAboutMe(ctx)
}

// ListExperiences returns all experiences ordered by display_order ASC, start_date DESC.
func (s *Store) ListExperiences(ctx context.Context) ([]models.Experience, error) {
	const q = `SELECT id, job_title, company_name, company_logo_url, location, start_date, end_date, description, icon_name, display_order, created_at, updated_at FROM experiences ORDER BY display_order ASC, start_date DESC, created_at DESC`
	rows, err := s.sqlDB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Experience
	for rows.Next() {
		var e models.Experience
		if err := rows.Scan(&e.ID, &e.JobTitle, &e.CompanyName, &e.CompanyLogoURL, &e.Location, &e.StartDate, &e.EndDate, &e.Description, &e.IconName, &e.DisplayOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

// GetExperience returns one experience by ID.
func (s *Store) GetExperience(ctx context.Context, id string) (*models.Experience, error) {
	const q = `SELECT id, job_title, company_name, company_logo_url, location, start_date, end_date, description, icon_name, display_order, created_at, updated_at FROM experiences WHERE id = ?`
	var e models.Experience
	err := s.sqlDB.QueryRowContext(ctx, q, id).Scan(&e.ID, &e.JobTitle, &e.CompanyName, &e.CompanyLogoURL, &e.Location, &e.StartDate, &e.EndDate, &e.Description, &e.IconName, &e.DisplayOrder, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CreateExperience inserts a new experience record.
func (s *Store) CreateExperience(ctx context.Context, exp models.Experience) (*models.Experience, error) {
	if exp.ID == "" {
		exp.ID = newUUID()
	}
	now := nowISO()
	exp.CreatedAt = now
	exp.UpdatedAt = now
	const q = `INSERT INTO experiences (id, job_title, company_name, company_logo_url, location, start_date, end_date, description, icon_name, display_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.sqlDB.ExecContext(ctx, q, exp.ID, exp.JobTitle, exp.CompanyName, exp.CompanyLogoURL, exp.Location, exp.StartDate, exp.EndDate, exp.Description, exp.IconName, exp.DisplayOrder, exp.CreatedAt, exp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &exp, nil
}

// UpdateExperience updates an existing experience by ID.
func (s *Store) UpdateExperience(ctx context.Context, id string, exp models.Experience) (*models.Experience, error) {
	exp.ID = id
	exp.UpdatedAt = nowISO()
	const q = `UPDATE experiences SET job_title = ?, company_name = ?, company_logo_url = ?, location = ?, start_date = ?, end_date = ?, description = ?, icon_name = ?, display_order = ?, updated_at = ? WHERE id = ?`
	res, err := s.sqlDB.ExecContext(ctx, q, exp.JobTitle, exp.CompanyName, exp.CompanyLogoURL, exp.Location, exp.StartDate, exp.EndDate, exp.Description, exp.IconName, exp.DisplayOrder, exp.UpdatedAt, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetExperience(ctx, id)
}

// DeleteExperience deletes an experience by ID.
func (s *Store) DeleteExperience(ctx context.Context, id string) error {
	res, err := s.sqlDB.ExecContext(ctx, `DELETE FROM experiences WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCertificates returns all certificates ordered by issue_date DESC, title ASC.
func (s *Store) ListCertificates(ctx context.Context) ([]models.Certificate, error) {
	const q = `SELECT id, title, issuing_organization, issue_date, credential_url, certificate_file_url, created_at FROM certificates ORDER BY issue_date DESC, title ASC`
	rows, err := s.sqlDB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Certificate
	for rows.Next() {
		var c models.Certificate
		if err := rows.Scan(&c.ID, &c.Title, &c.IssuingOrganization, &c.IssueDate, &c.CredentialURL, &c.CertificateFileURL, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// GetCertificate returns one certificate by ID.
func (s *Store) GetCertificate(ctx context.Context, id string) (*models.Certificate, error) {
	const q = `SELECT id, title, issuing_organization, issue_date, credential_url, certificate_file_url, created_at FROM certificates WHERE id = ?`
	var c models.Certificate
	err := s.sqlDB.QueryRowContext(ctx, q, id).Scan(&c.ID, &c.Title, &c.IssuingOrganization, &c.IssueDate, &c.CredentialURL, &c.CertificateFileURL, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCertificate inserts a new certificate record.
func (s *Store) CreateCertificate(ctx context.Context, c models.Certificate) (*models.Certificate, error) {
	if c.ID == "" {
		c.ID = newUUID()
	}
	c.CreatedAt = nowISO()
	const q = `INSERT INTO certificates (id, title, issuing_organization, issue_date, credential_url, certificate_file_url, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.sqlDB.ExecContext(ctx, q, c.ID, c.Title, c.IssuingOrganization, c.IssueDate, c.CredentialURL, c.CertificateFileURL, c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCertificate updates an existing certificate by ID.
func (s *Store) UpdateCertificate(ctx context.Context, id string, c models.Certificate) (*models.Certificate, error) {
	const q = `UPDATE certificates SET title = ?, issuing_organization = ?, issue_date = ?, credential_url = ?, certificate_file_url = ? WHERE id = ?`
	res, err := s.sqlDB.ExecContext(ctx, q, c.Title, c.IssuingOrganization, c.IssueDate, c.CredentialURL, c.CertificateFileURL, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetCertificate(ctx, id)
}

// DeleteCertificate deletes a certificate by ID.
func (s *Store) DeleteCertificate(ctx context.Context, id string) error {
	res, err := s.sqlDB.ExecContext(ctx, `DELETE FROM certificates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListProjects returns all projects ordered by display_order ASC, created_at DESC.
func (s *Store) ListProjects(ctx context.Context) ([]models.Project, error) {
	const q = `SELECT id, title, description, image_url, project_link, source_code_link, technologies_json, display_order, created_at, updated_at FROM projects ORDER BY display_order ASC, created_at DESC`
	rows, err := s.sqlDB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Project
	for rows.Next() {
		var p models.Project
		var techJSON string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.ImageURL, &p.ProjectLink, &p.SourceCodeLink, &techJSON, &p.DisplayOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(techJSON), &p.Technologies)
		items = append(items, p)
	}
	return items, rows.Err()
}

// GetProject returns one project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (*models.Project, error) {
	const q = `SELECT id, title, description, image_url, project_link, source_code_link, technologies_json, display_order, created_at, updated_at FROM projects WHERE id = ?`
	var p models.Project
	var techJSON string
	err := s.sqlDB.QueryRowContext(ctx, q, id).Scan(&p.ID, &p.Title, &p.Description, &p.ImageURL, &p.ProjectLink, &p.SourceCodeLink, &techJSON, &p.DisplayOrder, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(techJSON), &p.Technologies)
	return &p, nil
}

// CreateProject inserts a new project record.
func (s *Store) CreateProject(ctx context.Context, p models.Project) (*models.Project, error) {
	if p.ID == "" {
		p.ID = newUUID()
	}
	now := nowISO()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.Technologies = normalizeTags(p.Technologies)
	techBytes, _ := json.Marshal(p.Technologies)
	const q = `INSERT INTO projects (id, title, description, image_url, project_link, source_code_link, technologies_json, display_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.sqlDB.ExecContext(ctx, q, p.ID, p.Title, p.Description, p.ImageURL, p.ProjectLink, p.SourceCodeLink, string(techBytes), p.DisplayOrder, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProject updates an existing project by ID.
func (s *Store) UpdateProject(ctx context.Context, id string, p models.Project) (*models.Project, error) {
	p.UpdatedAt = nowISO()
	p.Technologies = normalizeTags(p.Technologies)
	techBytes, _ := json.Marshal(p.Technologies)
	const q = `UPDATE projects SET title = ?, description = ?, image_url = ?, project_link = ?, source_code_link = ?, technologies_json = ?, display_order = ?, updated_at = ? WHERE id = ?`
	res, err := s.sqlDB.ExecContext(ctx, q, p.Title, p.Description, p.ImageURL, p.ProjectLink, p.SourceCodeLink, string(techBytes), p.DisplayOrder, p.UpdatedAt, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetProject(ctx, id)
}

// DeleteProject deletes a project by ID.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.sqlDB.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func sortIconItems(items []models.IconItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})
}

func normalizeIconItems(items []models.IconItem) []models.IconItem {
	out := make([]models.IconItem, 0, len(items))
	for i, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		order := item.Order
		if order == 0 && i > 0 {
			order = i
		}
		out = append(out, models.IconItem{
			Name:     name,
			IconName: strings.TrimSpace(item.IconName),
			Order:    order,
		})
	}
	return out
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
