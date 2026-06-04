package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"imagestudio/internal/config"
)

func TestBase64ImageFromProviderItem(t *testing.T) {
	cases := []struct {
		name string
		item map[string]any
		want string
	}{
		{"prefers b64_json", map[string]any{"b64_json": "iVBORw0KGgo="}, "iVBORw0KGgo="},
		{"falls back to data uri url", map[string]any{"url": "data:image/png;base64,iVBORw0KGgo="}, "data:image/png;base64,iVBORw0KGgo="},
		{"ignores plain http url", map[string]any{"url": "https://cdn.example.com/cat.png"}, ""},
		{"b64_json wins over url", map[string]any{"b64_json": "AAAA", "url": "data:image/png;base64,BBBB"}, "AAAA"},
		{"empty when nothing usable", map[string]any{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base64ImageFromProviderItem(tc.item); got != tc.want {
				t.Fatalf("base64ImageFromProviderItem(%v) = %q, want %q", tc.item, got, tc.want)
			}
		})
	}
}

func TestPersistProviderImageResponseStoresDataURIImage(t *testing.T) {
	cfg := config.New(t.TempDir())
	cfg.Storage.ImageDir = t.TempDir()
	srv := &Server{cfg: cfg}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3, 4, 5}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	body := []byte(`{"data":[{"url":"` + dataURI + `"}]}`)

	out, stats := srv.persistProviderImageResponse(context.Background(), body, "user_duri", "conv_duri", "gen_duri")
	if stats.ImageCount != 1 {
		t.Fatalf("ImageCount = %d, want 1 (data uri image should be persisted)", stats.ImageCount)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal persisted response: %v", err)
	}
	data, _ := parsed["data"].([]any)
	if len(data) == 0 {
		t.Fatalf("persisted response has no data items")
	}
	item, _ := data[0].(map[string]any)
	url, _ := item["url"].(string)
	if !strings.HasPrefix(url, "/v1/files/image/") {
		t.Fatalf("persisted url = %q, want /v1/files/image/ prefix", url)
	}
}
