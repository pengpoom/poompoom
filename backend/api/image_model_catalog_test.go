package api

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"imagestudio/internal/businessproviders"
)

func TestStripModelVendorPrefix(t *testing.T) {
	cases := map[string]string{
		"openai/gpt-image-2":            "gpt-image-2",
		"google/gemini-2.5-flash-image": "gemini-2.5-flash-image",
		"gemini-2.5-flash-image":        "gemini-2.5-flash-image",
		"":                              "",
		"  openai/gpt-image-2  ":        "gpt-image-2",
		"a/b/c":                         "b/c",
	}
	for in, want := range cases {
		if got := stripModelVendorPrefix(in); got != want {
			t.Errorf("stripModelVendorPrefix(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolveBusinessImageModelPrefix(t *testing.T) {
	if strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN")) == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server, _, _, _ := provisionFacadeServer(t, ctx, "http://upstream.invalid")

	for _, model := range []string{
		"openai/gemini-2.5-flash-image",
		"google/gemini-2.5-flash-image",
		"gemini-2.5-flash-image",
	} {
		item, ok := server.resolveBusinessImageModel(ctx, map[string]any{"model": model})
		if !ok {
			t.Errorf("model %q: resolve failed", model)
			continue
		}
		if item.Platform != businessproviders.PlatformGeminiBanana {
			t.Errorf("model %q: platform=%q want %q", model, item.Platform, businessproviders.PlatformGeminiBanana)
		}
		if item.UpstreamModel != "gemini-2.5-flash-image" {
			t.Errorf("model %q: upstream=%q want gemini-2.5-flash-image", model, item.UpstreamModel)
		}
	}
}
