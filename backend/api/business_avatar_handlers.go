package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"imagestudio/internal/businessauth"
)

const (
	defaultAvatarDir             = "data/avatars"
	maxBusinessAvatarBytes       = 4 << 20
	maxBusinessAvatarRequestSize = maxBusinessAvatarBytes + (1 << 20)
)

func (s *Server) handleUploadBusinessMeAvatar(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBusinessAvatarRequestSize)
	if err := r.ParseMultipartForm(maxBusinessAvatarBytes + 1024); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid avatar upload"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "avatar file is required"})
		return
	}
	defer file.Close()

	limited := io.LimitReader(file, maxBusinessAvatarBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "read avatar failed"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "avatar file is empty"})
		return
	}
	if len(data) > maxBusinessAvatarBytes {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "avatar file is too large"})
		return
	}
	ext := avatarExtensionFromContent(data)
	if ext == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "avatar must be png, jpg, webp or gif"})
		return
	}

	name, err := newAvatarFileName(session.UserID, ext)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create avatar name failed"})
		return
	}
	dir := s.avatarStorageDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "avatar storage failed"})
		return
	}
	path := filepath.Join(dir, name)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save avatar failed"})
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save avatar failed"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	user, ok, err := store.UpdateUserAvatar(r.Context(), session.UserID, businessAvatarURL(name))
	if err != nil {
		_ = os.Remove(path)
		status := http.StatusBadRequest
		if errors.Is(err, businessauth.ErrUserDeleted) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		_ = os.Remove(path)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleBusinessAvatarFile(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimSpace(r.PathValue("name")))
	if name == "" || name != strings.TrimSpace(r.PathValue("name")) || !isAllowedImageFileName(name) {
		writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	path := filepath.Join(s.avatarStorageDir(), name)
	cleanPath := filepath.Clean(path)
	if filepath.Dir(cleanPath) != s.avatarStorageDir() {
		writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	info, err := os.Stat(cleanPath)
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	contentType := allowedImageFileExtensions[strings.ToLower(filepath.Ext(cleanPath))]
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Add("Vary", "Cookie")
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, cleanPath)
}

func (s *Server) avatarStorageDir() string {
	if s == nil || s.cfg == nil {
		return defaultAvatarDir
	}
	return filepath.Clean(s.cfg.ResolvePath(defaultAvatarDir))
}

func avatarExtensionFromContent(data []byte) string {
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	if len(data) >= 6 {
		signature := string(data[:6])
		if signature == "GIF87a" || signature == "GIF89a" {
			return ".gif"
		}
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	return ""
}

func newAvatarFileName(userID string, ext string) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	userPart := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(strings.TrimSpace(userID))
	if userPart == "" {
		userPart = "user"
	}
	return fmt.Sprintf("avatar-%s-%s%s", userPart, hex.EncodeToString(raw[:]), ext), nil
}

func businessAvatarURL(name string) string {
	return "/api/business/avatars/" + name
}
