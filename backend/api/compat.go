package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type compatChatCompletionRequest struct {
	Stream bool `json:"stream"`
}

type compatResponseRequest struct {
	Tools  []compatResponseTool `json:"tools"`
	Stream bool                 `json:"stream"`
}

type compatResponseTool struct {
	Type string `json:"type"`
}

func (s *Server) handleImageChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.rejectIfMaintenanceMode(w) {
		return
	}
	var req compatChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "", "invalid request body")
		return
	}
	if req.Stream {
		writeAPIError(w, http.StatusBadRequest, "stream_not_supported", "stream is not supported for image generation")
		return
	}

	writeImageCompatRemoved(w)
}

func (s *Server) handleImageResponses(w http.ResponseWriter, r *http.Request) {
	if s.rejectIfMaintenanceMode(w) {
		return
	}
	var req compatResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "", "invalid request body")
		return
	}
	if req.Stream {
		writeAPIError(w, http.StatusBadRequest, "stream_not_supported", "stream is not supported for image generation")
		return
	}
	if !hasCompatImageGenerationTool(req.Tools) {
		writeAPIError(w, http.StatusBadRequest, "image_generation_tool_required", "only image_generation tool requests are supported on this endpoint")
		return
	}

	writeImageCompatRemoved(w)
}

func hasCompatImageGenerationTool(tools []compatResponseTool) bool {
	for _, tool := range tools {
		if strings.EqualFold(strings.TrimSpace(tool.Type), "image_generation") {
			return true
		}
	}
	return false
}

func writeImageCompatRemoved(w http.ResponseWriter) {
	writeAPIError(w, http.StatusGone, "image_compat_removed", "图片兼容接口已下线，请使用网页生图入口")
}
