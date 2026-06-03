package riskcontrol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type openAIModerationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type openAIModerationInputItem struct {
	Type     string                       `json:"type"`
	Text     string                       `json:"text,omitempty"`
	ImageURL *openAIModerationImageURLRef `json:"image_url,omitempty"`
}

type openAIModerationImageURLRef struct {
	URL string `json:"url"`
}

type openAIModerationResponse struct {
	Results []struct {
		Flagged        bool               `json:"flagged"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func (s *Service) moderateOpenAI(ctx context.Context, cfg Config, input string, images []ModerationImage) (Decision, int64, error) {
	cfg = NormalizeConfig(cfg)
	if cfg.APIKey == "" {
		return Decision{}, 0, newRiskControlConfigError("risk control api key is not configured")
	}
	endpoint, err := openAIModerationEndpoint(cfg.BaseURL)
	if err != nil {
		return Decision{}, 0, wrapRiskControlConfigError(err)
	}
	moderationInput, err := openAIModerationInput(input, images)
	if err != nil {
		return Decision{}, 0, err
	}
	raw, err := json.Marshal(openAIModerationRequest{
		Model: cfg.Model,
		Input: moderationInput,
	})
	if err != nil {
		return Decision{}, 0, err
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return Decision{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	startedAt := time.Now()
	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	latencyMS := time.Since(startedAt).Milliseconds()
	if err != nil {
		return Decision{}, latencyMS, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Decision{}, latencyMS, err
	}
	var parsed openAIModerationResponse
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			message = parsed.Error.Message
		}
		err := fmt.Errorf("moderation upstream returned %d: %s", resp.StatusCode, message)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return Decision{}, latencyMS, wrapRiskControlConfigError(err)
		}
		return Decision{}, latencyMS, err
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Decision{}, latencyMS, err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return Decision{}, latencyMS, errors.New(parsed.Error.Message)
	}
	if len(parsed.Results) == 0 {
		return Decision{}, latencyMS, errors.New("moderation upstream returned no result")
	}
	scores := parsed.Results[0].CategoryScores
	if scores == nil {
		scores = map[string]float64{}
	}
	return decisionFromScores(parsed.Results[0].Flagged, scores, cfg.Thresholds), latencyMS, nil
}

func openAIModerationInput(input string, images []ModerationImage) (any, error) {
	input = normalizeInputText(input)
	images = normalizeImages(images)
	if len(images) == 0 {
		return input, nil
	}
	items := make([]openAIModerationInputItem, 0, len(images)+1)
	if input != "" {
		items = append(items, openAIModerationInputItem{
			Type: "text",
			Text: input,
		})
	}
	for _, image := range images {
		if len(image.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(image.MimeType)
		if mimeType == "" {
			mimeType = http.DetectContentType(image.Data)
		}
		if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
			return nil, fmt.Errorf("unsupported moderation image mime type %q", mimeType)
		}
		items = append(items, openAIModerationInputItem{
			Type: "image_url",
			ImageURL: &openAIModerationImageURLRef{
				URL: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image.Data),
			},
		})
	}
	if len(items) == 0 {
		return input, nil
	}
	return items, nil
}

func openAIModerationEndpoint(baseURL string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid moderation base url %q", baseURL)
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/moderations") {
		return parsed.String(), nil
	}
	if strings.HasSuffix(path, "/v1") {
		parsed.Path = path + "/moderations"
		return parsed.String(), nil
	}
	parsed.Path = path + "/v1/moderations"
	return parsed.String(), nil
}
