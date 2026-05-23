package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleImageResponsesReturnsGoneAfterCompatImageRemoval(t *testing.T) {
	server := &Server{}
	body := `{
		"model":"gpt-image-2",
		"input":[
			{
				"role":"user",
				"content":[
					{"type":"input_text","text":"请编辑这张图"},
					{"type":"input_image","image_url":"http://127.0.0.1:7000/secret"}
				]
			}
		],
		"tools":[{"type":"image_generation"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/responses", strings.NewReader(body))
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	server.handleImageResponses(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
	if !strings.Contains(rec.Body.String(), "image_compat_removed") {
		t.Fatalf("body = %s, want image_compat_removed code", rec.Body.String())
	}
}
