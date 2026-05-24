package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"imagestudio/internal/businessimage"
)

func (s *Server) handleListBusinessImageConversations(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_store_failed", err.Error())
		return
	}
	defer store.Close()

	userID := businessUserIDForRequest(r)
	items, err := store.ListConversations(r.Context(), userID, businessImageLimit(r, 50))
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"userId": userID,
		"items":  items,
	})
}

func (s *Server) handleGetBusinessImageConversation(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_store_failed", err.Error())
		return
	}
	defer store.Close()

	item, ok, err := store.GetConversationWithGenerations(
		r.Context(),
		r.PathValue("id"),
		businessUserIDForRequest(r),
		businessImageLimit(r, 100),
	)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_get_failed", err.Error())
		return
	}
	if !ok {
		writeAPIError(w, http.StatusNotFound, "business_image_not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleRenameBusinessImageConversation(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		writeAPIError(w, http.StatusBadRequest, "business_image_title_required", "conversation title is required")
		return
	}
	if len([]rune(title)) > 80 {
		writeAPIError(w, http.StatusBadRequest, "business_image_title_too_long", "conversation title is too long")
		return
	}

	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_store_failed", err.Error())
		return
	}
	defer store.Close()

	item, ok, err := store.RenameConversation(
		r.Context(),
		r.PathValue("id"),
		businessUserIDForRequest(r),
		title,
	)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_rename_failed", err.Error())
		return
	}
	if !ok {
		writeAPIError(w, http.StatusNotFound, "business_image_not_found", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteBusinessImageConversation(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_store_failed", err.Error())
		return
	}
	defer store.Close()

	userID := businessUserIDForRequest(r)
	fileNames, err := store.ImageFileNamesForConversation(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_delete_failed", err.Error())
		return
	}
	result, err := store.DeleteConversation(
		r.Context(),
		r.PathValue("id"),
		userID,
	)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_delete_failed", err.Error())
		return
	}
	if result.Conversations == 0 {
		writeAPIError(w, http.StatusNotFound, "business_image_not_found", "conversation not found")
		return
	}
	deletedFiles, err := s.deleteUnreferencedBusinessImageFiles(r, store, fileNames)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_file_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"userId":       userID,
		"result":       result,
		"deletedFiles": deletedFiles,
	})
}

func (s *Server) handleClearBusinessImageConversations(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessImageStore()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_store_failed", err.Error())
		return
	}
	defer store.Close()

	userID := businessUserIDForRequest(r)
	fileNames, err := store.ImageFileNamesForUser(r.Context(), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_clear_failed", err.Error())
		return
	}
	result, err := store.ClearConversations(r.Context(), userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_clear_failed", err.Error())
		return
	}
	deletedFiles, err := s.deleteUnreferencedBusinessImageFiles(r, store, fileNames)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "business_image_file_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"userId":       userID,
		"result":       result,
		"deletedFiles": deletedFiles,
	})
}

func (s *Server) deleteUnreferencedBusinessImageFiles(r *http.Request, store *businessimage.Store, fileNames []string) (int, error) {
	seen := map[string]struct{}{}
	deleted := 0
	for _, fileName := range fileNames {
		cleaned := filepath.Base(strings.TrimSpace(fileName))
		if cleaned == "" || !strings.HasPrefix(cleaned, "business-") {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		referenced, err := store.ImageFileNameReferenced(r.Context(), cleaned)
		if err != nil {
			return deleted, err
		}
		if referenced {
			continue
		}
		path := s.businessImageFilePath(cleaned)
		if path == "" {
			if err := store.DeleteAssetsByFileName(r.Context(), cleaned); err != nil {
				return deleted, err
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				if err := store.DeleteAssetsByFileName(r.Context(), cleaned); err != nil {
					return deleted, err
				}
				continue
			}
			return deleted, err
		}
		if err := store.DeleteAssetsByFileName(r.Context(), cleaned); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *Server) businessImageFilePath(fileName string) string {
	cleaned := filepath.Base(strings.TrimSpace(fileName))
	if cleaned == "" || !strings.HasPrefix(cleaned, "business-") {
		return ""
	}
	for _, dir := range s.businessImageStorageDirs() {
		path := filepath.Join(dir, cleaned)
		cleanPath := filepath.Clean(path)
		if filepath.Dir(cleanPath) != dir {
			continue
		}
		_, err := os.Stat(cleanPath)
		if err == nil {
			return cleanPath
		}
		if os.IsNotExist(err) {
			continue
		}
	}
	return ""
}

func (s *Server) businessImageStorageDirs() []string {
	seen := map[string]struct{}{}
	dirs := make([]string, 0, 2)
	for _, raw := range []string{s.cfg.Storage.ImageDir, legacyImageDir} {
		resolved := filepath.Clean(s.cfg.ResolvePath(raw))
		if resolved == "" {
			continue
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		dirs = append(dirs, resolved)
	}
	return dirs
}

func businessImageLimit(r *http.Request, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	if value > 200 {
		return 200
	}
	return value
}
