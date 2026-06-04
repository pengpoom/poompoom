package api

import (
	"net/http"

	"imagestudio/internal/businessimage"
)

type businessAssetsResponse struct {
	Items []businessimage.AssetDetail `json:"items"`
	Page  paginationMeta              `json:"page"`
}

func (s *Server) handleListBusinessAssets(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok || session.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	page, pageSize, offset := paginationFromQuery(r, "", 8, 50)
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "asset store failed"})
		return
	}
	defer imageStore.Close()

	items, total, err := imageStore.AssetDetailsByUserPage(r.Context(), session.UserID, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, businessAssetsResponse{
		Items: items,
		Page:  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}
