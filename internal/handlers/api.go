package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/NeddyAP/go-portfolio/internal/db"
	"github.com/NeddyAP/go-portfolio/internal/github"
	"github.com/NeddyAP/go-portfolio/internal/models"
)

func (s *Server) handleUpdateAboutMe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	var upd models.AboutMeUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if upd.ContactEmail != nil {
		if err := ValidateEmail(*upd.ContactEmail); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid contact email address")
			return
		}
	}
	for _, pair := range []struct {
		name string
		val  *string
	}{
		{"profile_image_url", upd.ProfileImageURL},
		{"resume_url", upd.ResumeURL},
		{"github_url", upd.GitHubURL},
		{"linkedin_url", upd.LinkedInURL},
	} {
		if pair.val != nil {
			if err := ValidateSafeURL(*pair.val, true); err != nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid "+pair.name+": "+err.Error())
				return
			}
		}
	}

	prev, _ := s.store.GetAboutMe(r.Context())
	updated, err := s.store.UpdateAboutMe(r.Context(), upd)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update About Me")
		return
	}

	s.cache.Invalidate()
	if prev != nil && updated.GitHubURL != prev.GitHubURL {
		if username := github.ParseUsername(updated.GitHubURL); username != "" {
			s.github.WarmAsync(username)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "About Me updated successfully",
		"data":    updated,
	})
}

func (s *Server) handleCreateExperience(w http.ResponseWriter, r *http.Request) {
	exp, ok := decodeAndValidateExperience(w, r)
	if !ok {
		return
	}
	created, err := s.store.CreateExperience(r.Context(), exp)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create experience")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateExperience(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing experience ID")
		return
	}
	exp, ok := decodeAndValidateExperience(w, r)
	if !ok {
		return
	}
	updated, err := s.store.UpdateExperience(r.Context(), id, exp)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Experience not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update experience")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteExperience(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing experience ID")
		return
	}
	err := s.store.DeleteExperience(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Experience not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete experience")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeAndValidateExperience(w http.ResponseWriter, r *http.Request) (models.Experience, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	var exp models.Experience
	if err := json.NewDecoder(r.Body).Decode(&exp); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return exp, false
	}
	exp.JobTitle = strings.TrimSpace(exp.JobTitle)
	exp.CompanyName = strings.TrimSpace(exp.CompanyName)
	exp.CompanyLogoURL = strings.TrimSpace(exp.CompanyLogoURL)
	exp.Location = strings.TrimSpace(exp.Location)
	exp.StartDate = strings.TrimSpace(exp.StartDate)
	exp.EndDate = strings.TrimSpace(exp.EndDate)
	exp.Description = strings.TrimSpace(exp.Description)
	exp.IconName = strings.TrimSpace(exp.IconName)

	if exp.JobTitle == "" || exp.CompanyName == "" {
		writeJSONError(w, http.StatusBadRequest, "Job title and company name are required")
		return exp, false
	}
	if err := ValidateSafeURL(exp.CompanyLogoURL, true); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid company logo URL: "+err.Error())
		return exp, false
	}
	return exp, true
}

func (s *Server) handleCreateCertificate(w http.ResponseWriter, r *http.Request) {
	cert, ok := decodeAndValidateCertificate(w, r)
	if !ok {
		return
	}
	created, err := s.store.CreateCertificate(r.Context(), cert)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create certificate")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateCertificate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing certificate ID")
		return
	}
	cert, ok := decodeAndValidateCertificate(w, r)
	if !ok {
		return
	}
	updated, err := s.store.UpdateCertificate(r.Context(), id, cert)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Certificate not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update certificate")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteCertificate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing certificate ID")
		return
	}
	existing, err := s.store.GetCertificate(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Certificate not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load certificate")
		return
	}
	if err := s.store.DeleteCertificate(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete certificate")
		return
	}
	if existing.CertificateFileURL != "" {
		_ = s.removeUploadedFile(existing.CertificateFileURL, "certificate")
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeAndValidateCertificate(w http.ResponseWriter, r *http.Request) (models.Certificate, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	var cert models.Certificate
	if err := json.NewDecoder(r.Body).Decode(&cert); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return cert, false
	}
	cert.Title = strings.TrimSpace(cert.Title)
	cert.IssuingOrganization = strings.TrimSpace(cert.IssuingOrganization)
	cert.IssueDate = strings.TrimSpace(cert.IssueDate)
	cert.CredentialURL = strings.TrimSpace(cert.CredentialURL)
	cert.CertificateFileURL = strings.TrimSpace(cert.CertificateFileURL)

	if cert.Title == "" || cert.IssuingOrganization == "" {
		writeJSONError(w, http.StatusBadRequest, "Title and issuing organization are required")
		return cert, false
	}
	if err := ValidateSafeURL(cert.CredentialURL, false); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid credential URL: "+err.Error())
		return cert, false
	}
	if err := ValidateSafeURL(cert.CertificateFileURL, true); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid certificate file URL: "+err.Error())
		return cert, false
	}
	return cert, true
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	proj, ok := decodeAndValidateProject(w, r)
	if !ok {
		return
	}
	created, err := s.store.CreateProject(r.Context(), proj)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create project")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing project ID")
		return
	}
	proj, ok := decodeAndValidateProject(w, r)
	if !ok {
		return
	}
	updated, err := s.store.UpdateProject(r.Context(), id, proj)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Project not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update project")
		return
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing project ID")
		return
	}
	existing, err := s.store.GetProject(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Project not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load project")
		return
	}
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete project")
		return
	}
	if existing.ImageURL != "" {
		_ = s.removeUploadedFile(existing.ImageURL, "project")
	}
	s.cache.Invalidate()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeAndValidateProject(w http.ResponseWriter, r *http.Request) (models.Project, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	var proj models.Project
	if err := json.NewDecoder(r.Body).Decode(&proj); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return proj, false
	}
	proj.Title = strings.TrimSpace(proj.Title)
	proj.Description = strings.TrimSpace(proj.Description)
	proj.ImageURL = strings.TrimSpace(proj.ImageURL)
	proj.ProjectLink = strings.TrimSpace(proj.ProjectLink)
	proj.SourceCodeLink = strings.TrimSpace(proj.SourceCodeLink)

	if proj.Title == "" {
		writeJSONError(w, http.StatusBadRequest, "Project title is required")
		return proj, false
	}
	if err := ValidateSafeURL(proj.ImageURL, true); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid image URL: "+err.Error())
		return proj, false
	}
	if err := ValidateSafeURL(proj.ProjectLink, false); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid project link: "+err.Error())
		return proj, false
	}
	if err := ValidateSafeURL(proj.SourceCodeLink, false); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid source code link: "+err.Error())
		return proj, false
	}
	return proj, true
}
