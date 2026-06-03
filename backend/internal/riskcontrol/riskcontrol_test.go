package riskcontrol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNormalizeConfigAppliesDefaultsAndBounds(t *testing.T) {
	cfg := NormalizeConfig(Config{
		Mode:         "PRE_BLOCK",
		Provider:     "unknown",
		BaseURL:      " https://moderation.example/v1/ ",
		APIKey:       " secret ",
		Model:        " ",
		TimeoutMS:    60000,
		BlockMessage: " ",
		Thresholds: map[string]float64{
			" Violence ": 1.2,
			"custom":     -0.2,
			" ":          0.5,
		},
	})

	if cfg.Mode != ModePreBlock {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModePreBlock)
	}
	if cfg.Provider != ProviderOpenAI {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, ProviderOpenAI)
	}
	if cfg.BaseURL != "https://moderation.example/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "secret" {
		t.Fatalf("APIKey was not trimmed")
	}
	if cfg.Model != DefaultConfig().Model {
		t.Fatalf("Model = %q, want default", cfg.Model)
	}
	if cfg.TimeoutMS != 30000 {
		t.Fatalf("TimeoutMS = %d, want 30000", cfg.TimeoutMS)
	}
	if cfg.BlockMessage != DefaultConfig().BlockMessage {
		t.Fatalf("BlockMessage = %q, want default", cfg.BlockMessage)
	}
	if cfg.Thresholds["violence"] != 1 {
		t.Fatalf("violence threshold = %v, want 1", cfg.Thresholds["violence"])
	}
	if cfg.Thresholds["custom"] != 0 {
		t.Fatalf("custom threshold = %v, want 0", cfg.Thresholds["custom"])
	}
	if _, ok := cfg.Thresholds[" "]; ok {
		t.Fatal("blank threshold key was kept")
	}
}

func TestDecisionFromScoresUsesThresholdsAndHighestScore(t *testing.T) {
	decision := decisionFromScores(false, map[string]float64{
		"sexual":   0.40,
		"violence": 0.96,
	}, map[string]float64{
		"violence": 0.95,
	})

	if !decision.Flagged {
		t.Fatal("Flagged = false, want true")
	}
	if decision.HighestCategory != "violence" || decision.HighestScore != 0.96 {
		t.Fatalf("highest = %q/%v, want violence/0.96", decision.HighestCategory, decision.HighestScore)
	}
	if !decision.Allowed || decision.Action != ActionAllow {
		t.Fatalf("decision allowed/action = %v/%q", decision.Allowed, decision.Action)
	}
}

func TestApplyTestOverrideUsesExplicitEmptyValues(t *testing.T) {
	baseURL := ""
	model := ""
	blockMessage := ""
	timeoutMS := 0
	thresholds := map[string]float64{}

	cfg := applyTestOverride(Config{
		BaseURL:      "https://moderation.example",
		APIKey:       "saved-key",
		Model:        "saved-model",
		BlockMessage: "saved block",
		TimeoutMS:    9000,
		Thresholds: map[string]float64{
			"violence": 0.2,
		},
	}, TestConfigOverride{
		BaseURL:      &baseURL,
		Model:        &model,
		BlockMessage: &blockMessage,
		TimeoutMS:    &timeoutMS,
		Thresholds:   &thresholds,
	})

	if cfg.BaseURL != DefaultConfig().BaseURL {
		t.Fatalf("BaseURL = %q, want default", cfg.BaseURL)
	}
	if cfg.Model != DefaultConfig().Model {
		t.Fatalf("Model = %q, want default", cfg.Model)
	}
	if cfg.BlockMessage != DefaultConfig().BlockMessage {
		t.Fatalf("BlockMessage = %q, want default", cfg.BlockMessage)
	}
	if cfg.TimeoutMS != DefaultConfig().TimeoutMS {
		t.Fatalf("TimeoutMS = %d, want default", cfg.TimeoutMS)
	}
	if cfg.Thresholds["violence"] != DefaultConfig().Thresholds["violence"] {
		t.Fatalf("violence threshold = %v, want default", cfg.Thresholds["violence"])
	}
}

func TestApplyTestOverrideCanClearAPIKey(t *testing.T) {
	cfg := applyTestOverride(Config{APIKey: "saved-key"}, TestConfigOverride{ClearAPIKey: true})
	if cfg.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", cfg.APIKey)
	}
}

func TestErrorDecisionClassifiesConfigErrors(t *testing.T) {
	decision := ErrorDecision(newRiskControlConfigError("missing api key"))

	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if decision.Action != ActionError {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionError)
	}
	if decision.InternalErrorCode != DecisionErrorConfig {
		t.Fatalf("InternalErrorCode = %q, want %q", decision.InternalErrorCode, DecisionErrorConfig)
	}
	if !strings.Contains(decision.Error, "missing api key") {
		t.Fatalf("Error = %q, want missing api key", decision.Error)
	}
}

func TestErrorDecisionClassifiesUnavailableErrors(t *testing.T) {
	decision := ErrorDecision(context.DeadlineExceeded)

	if decision.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if decision.Action != ActionError {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionError)
	}
	if decision.InternalErrorCode != DecisionErrorUnavailable {
		t.Fatalf("InternalErrorCode = %q, want %q", decision.InternalErrorCode, DecisionErrorUnavailable)
	}
}

func TestModerateOpenAIMissingAPIKeyIsConfigError(t *testing.T) {
	service := &Service{}

	_, _, err := service.moderateOpenAI(context.Background(), Config{}, "prompt", nil)
	if err == nil {
		t.Fatal("moderateOpenAI() error = nil, want config error")
	}
	decision := ErrorDecision(err)
	if decision.InternalErrorCode != DecisionErrorConfig {
		t.Fatalf("InternalErrorCode = %q, want %q", decision.InternalErrorCode, DecisionErrorConfig)
	}
}

func TestModerateOpenAIUnauthorizedPlainTextIsConfigError(t *testing.T) {
	service := &Service{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("invalid api key")),
				}, nil
			}),
		},
	}

	_, _, err := service.moderateOpenAI(context.Background(), Config{
		BaseURL:   "https://moderation.example",
		APIKey:    "bad-key",
		TimeoutMS: 1000,
	}, "prompt", nil)
	if err == nil {
		t.Fatal("moderateOpenAI() error = nil, want config error")
	}
	decision := ErrorDecision(err)
	if decision.InternalErrorCode != DecisionErrorConfig {
		t.Fatalf("InternalErrorCode = %q, want %q", decision.InternalErrorCode, DecisionErrorConfig)
	}
}

func TestOpenAIModerationEndpoint(t *testing.T) {
	tests := map[string]string{
		"":                              "https://api.openai.com/v1/moderations",
		"https://moderation.example":    "https://moderation.example/v1/moderations",
		"https://moderation.example/v1": "https://moderation.example/v1/moderations",
		"https://moderation.example/v1/moderations":  "https://moderation.example/v1/moderations",
		"https://moderation.example/custom":          "https://moderation.example/custom/v1/moderations",
		"https://moderation.example/custom/":         "https://moderation.example/custom/v1/moderations",
		"https://moderation.example/custom?region=1": "https://moderation.example/custom/v1/moderations?region=1",
	}

	for input, want := range tests {
		got, err := openAIModerationEndpoint(input)
		if err != nil {
			t.Fatalf("openAIModerationEndpoint(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("openAIModerationEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestModerateOpenAIUsesRequestAndThresholds(t *testing.T) {
	service := &Service{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %q, want POST", req.Method)
				}
				if req.URL.String() != "https://moderation.example/v1/moderations" {
					t.Fatalf("url = %q", req.URL.String())
				}
				if req.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
				}
				var payload openAIModerationRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if payload.Model != "omni-moderation-test" || payload.Input != "blocked prompt" {
					t.Fatalf("payload = %#v", payload)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"results": [{
							"flagged": false,
							"category_scores": {
								"violence": 0.96,
								"sexual": 0.1
							}
						}]
					}`)),
				}, nil
			}),
		},
	}

	decision, latencyMS, err := service.moderateOpenAI(context.Background(), Config{
		BaseURL:   "https://moderation.example",
		APIKey:    "test-key",
		Model:     "omni-moderation-test",
		TimeoutMS: 1000,
		Thresholds: map[string]float64{
			"violence": 0.95,
		},
	}, "blocked prompt", nil)
	if err != nil {
		t.Fatalf("moderateOpenAI error: %v", err)
	}
	if latencyMS < 0 {
		t.Fatalf("latencyMS = %d, want >= 0", latencyMS)
	}
	if !decision.Flagged || decision.HighestCategory != "violence" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestModerateOpenAIIncludesImages(t *testing.T) {
	imageBytes := []byte("reference-image")
	expectedImageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
	service := &Service{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var payload openAIModerationRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				items, ok := payload.Input.([]any)
				if !ok || len(items) != 2 {
					t.Fatalf("input = %#v, want text and image items", payload.Input)
				}
				textItem, ok := items[0].(map[string]any)
				if !ok || textItem["type"] != "text" || textItem["text"] != "prompt" {
					t.Fatalf("text item = %#v", items[0])
				}
				imageItem, ok := items[1].(map[string]any)
				if !ok || imageItem["type"] != "image_url" {
					t.Fatalf("image item = %#v", items[1])
				}
				imageURL, ok := imageItem["image_url"].(map[string]any)
				if !ok || imageURL["url"] != expectedImageURL {
					t.Fatalf("image_url = %#v, want %q", imageItem["image_url"], expectedImageURL)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"results": [{
							"flagged": false,
							"category_scores": {}
						}]
					}`)),
				}, nil
			}),
		},
	}

	_, _, err := service.moderateOpenAI(context.Background(), Config{
		BaseURL:   "https://moderation.example",
		APIKey:    "test-key",
		Model:     "omni-moderation-test",
		TimeoutMS: 1000,
	}, "prompt", []ModerationImage{{MimeType: "image/png", Data: imageBytes}})
	if err != nil {
		t.Fatalf("moderateOpenAI error: %v", err)
	}
}

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("short"); got != "***" {
		t.Fatalf("MaskSecret(short) = %q", got)
	}
	if got := MaskSecret("sk-1234567890"); got != "sk-1***7890" {
		t.Fatalf("MaskSecret(long) = %q", got)
	}
}
