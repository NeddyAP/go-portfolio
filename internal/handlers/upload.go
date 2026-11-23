package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleUploadPOST(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+128*1024)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes + 64*1024); err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "Upload exceeds maximum size limit")
		return
	}

	fileType := strings.TrimSpace(r.FormValue("type"))
	if !isAllowedUploadCategory(fileType) {
		writeJSONError(w, http.StatusBadRequest, "Invalid upload type; must be profile, resume, project, or certificate")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	limited := io.LimitReader(file, s.cfg.MaxUploadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Failed to read uploaded file")
		return
	}
	if int64(len(data)) > s.cfg.MaxUploadBytes {
		writeJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("File exceeds %dMB limit", s.cfg.MaxUploadBytes/(1024*1024)))
		return
	}
	if len(data) == 0 {
		writeJSONError(w, http.StatusBadRequest, "Uploaded file is empty")
		return
	}

	ext, err := sniffAllowedExtension(fileType, data)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	destDir := filepath.Join(s.cfg.DataDir, "uploads", fileType)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to prepare upload directory")
		return
	}

	randBytes := make([]byte, 12)
	_, _ = rand.Read(randBytes)
	destName := fileType + "-" + hex.EncodeToString(randBytes) + ext
	destPath := filepath.Join(destDir, destName)

	tmpFile, err := os.CreateTemp(destDir, ".upload-*")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save upload")
		return
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		writeJSONError(w, http.StatusInternalServerError, "Failed to write upload")
		return
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		writeJSONError(w, http.StatusInternalServerError, "Failed to finalize upload")
		return
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		writeJSONError(w, http.StatusInternalServerError, "Failed to set file permissions")
		return
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		_ = os.Remove(tmpName)
		writeJSONError(w, http.StatusInternalServerError, "Failed to commit upload")
		return
	}

	// Clean up replaced local upload if applicable (never touches embedded /static/* files).
	if currentURL := strings.TrimSpace(r.FormValue("currentFileUrl")); currentURL != "" {
		_ = s.removeUploadedFile(currentURL, fileType)
	}

	publicURL := "/uploads/" + fileType + "/" + destName
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "File uploaded successfully",
		"url":     publicURL,
	})
}

func (s *Server) handleUploadDELETE(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req struct {
		FileURL string `json:"fileUrl"`
		Type    string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	req.FileURL = strings.TrimSpace(req.FileURL)
	if !isAllowedUploadCategory(req.Type) {
		writeJSONError(w, http.StatusBadRequest, "Invalid upload type")
		return
	}

	_ = s.removeUploadedFile(req.FileURL, req.Type)
	writeJSON(w, http.StatusOK, map[string]string{"message": "File deleted"})
}

func sniffAllowedExtension(fileType string, data []byte) (string, error) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}

	switch fileType {
	case "profile", "project":
		ct := http.DetectContentType(head)
		switch ct {
		case "image/jpeg":
			return ".jpg", nil
		case "image/png":
			return ".png", nil
		case "image/gif":
			return ".gif", nil
		case "image/webp":
			return ".webp", nil
		default:
			return "", fmt.Errorf("invalid image format (%s); expected JPEG, PNG, GIF, or WebP", ct)
		}
	case "resume", "certificate":
		trimmed := bytes.TrimLeft(head, "\xef\xbb\xbf \t\r\n")
		if !bytes.HasPrefix(trimmed, []byte("%PDF-")) {
			return "", fmt.Errorf("invalid file format; expected a valid PDF document")
		}
		return ".pdf", nil
	default:
		return "", fmt.Errorf("unsupported upload type")
	}
}

func (s *Server) removeUploadedFile(rawURL, expectedCategory string) error {
	prefix := "/uploads/" + expectedCategory + "/"
	if !strings.HasPrefix(rawURL, prefix) {
		return nil
	}
	filename := strings.TrimPrefix(rawURL, prefix)
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("unsafe upload path")
	}
	target := filepath.Join(s.cfg.DataDir, "uploads", expectedCategory, filename)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
