package api

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businessimage"
)

const defaultImageDir = "data/business-images"
const legacyImageDir = "data/tmp/image"

var allowedImageFileExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// downloadAndCache downloads an upstream image using the image client's transport
// (Chrome TLS fingerprint), saves to local disk, and returns the local filename.
func downloadAndCache(client imageDownloader, upstreamURL string, cacheDir string) (string, error) {
	// Generate a stable filename from the URL
	hash := sha256.Sum256([]byte(upstreamURL))
	filename := fmt.Sprintf("%x.png", hash[:12])
	dir := firstNonEmpty(cacheDir, defaultImageDir)
	localPath := filepath.Join(dir, filename)

	// Check cache
	if _, err := os.Stat(localPath); err == nil {
		return filename, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	data, err := client.DownloadBytes(upstreamURL)
	if err != nil {
		return "", fmt.Errorf("download upstream image: %w", err)
	}

	tmpFile := localPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpFile, localPath); err != nil {
		return "", err
	}

	slog.Info("cached image", "file", filename, "size", len(data))
	return filename, nil
}

// gatewayImageURL builds the public URL for a cached image.
func gatewayImageURL(r *http.Request, filename string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	return fmt.Sprintf("%s://%s/v1/files/image/%s", scheme, host, filename)
}

func (s *Server) resolveImageFilePath(name string) string {
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "" || !isAllowedImageFileName(baseName) {
		return ""
	}
	if s == nil || s.cfg == nil {
		return ""
	}

	for _, dir := range s.businessImageStorageDirs() {
		candidate := filepath.Join(dir, baseName)
		cleanCandidate := filepath.Clean(candidate)
		if filepath.Dir(cleanCandidate) != dir {
			continue
		}
		info, err := os.Stat(cleanCandidate)
		if err == nil && info.Mode().IsRegular() {
			return cleanCandidate
		}
	}
	return ""
}

func isAllowedImageFileName(name string) bool {
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "" || baseName != strings.TrimSpace(name) {
		return false
	}
	_, ok := allowedImageFileExtensions[strings.ToLower(filepath.Ext(baseName))]
	return ok
}

// handleImageFile serves cached and server-stored images from storage.image_dir.
func (s *Server) handleImageFile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/v1/files/image/")
	name = strings.ReplaceAll(name, "/", "-")
	if name == "" {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	isBusinessImage := strings.HasPrefix(name, "business-")
	if isBusinessImage {
		if status, ok := s.authorizeBusinessImageFile(r, name); !ok {
			switch status {
			case http.StatusForbidden:
				writeError(w, status, "image access is forbidden")
			case http.StatusNotFound:
				writeError(w, status, "image not found")
			case http.StatusServiceUnavailable:
				writeError(w, status, "session store unavailable")
			case http.StatusInternalServerError:
				writeError(w, status, "image authorization failed")
			default:
				writeError(w, http.StatusUnauthorized, "authorization is invalid")
			}
			return
		}
	}

	path := s.resolveImageFilePath(name)
	if path == "" {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}

	ct := allowedImageFileExtensions[strings.ToLower(filepath.Ext(path))]
	if ct == "" {
		ct = "image/png"
	}

	if isBusinessImage {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.Header().Add("Vary", "Authorization")
		w.Header().Add("Vary", "Cookie")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.Header().Set("Content-Type", ct)
	http.ServeFile(w, r, path)
}

func (s *Server) authorizeBusinessImageFile(r *http.Request, name string) (int, bool) {
	session, ok, err := s.authSessionForRequest(r)
	if err != nil {
		return http.StatusServiceUnavailable, false
	}
	if !ok {
		return http.StatusUnauthorized, false
	}
	store, err := businessauth.NewStore(s.cfg)
	if err != nil {
		return http.StatusInternalServerError, false
	}
	defer store.Close()
	user, ok, err := store.GetUserByID(r.Context(), session.UserID)
	if err != nil || !ok {
		return http.StatusUnauthorized, false
	}
	if user.Status != businessauth.StatusActive {
		return http.StatusUnauthorized, false
	}
	if user.Role == authRoleAdmin {
		return http.StatusOK, true
	}

	imageStore, err := businessimage.NewStore(s.cfg)
	if err != nil {
		return http.StatusInternalServerError, false
	}
	defer imageStore.Close()
	owners, err := imageStore.ImageFileOwnerUserIDs(r.Context(), name)
	if err != nil {
		return http.StatusInternalServerError, false
	}
	if len(owners) == 0 {
		return http.StatusNotFound, false
	}
	for _, ownerID := range owners {
		if ownerID == session.UserID {
			return http.StatusOK, true
		}
	}
	return http.StatusForbidden, false
}
