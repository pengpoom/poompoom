package riskcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green "github.com/alibabacloud-go/green-20220302/v3/client"
	"github.com/alibabacloud-go/tea/dara"
)

const aliyunMaxContentRunes = 2000

type aliyunTextModerationPayload struct {
	DataID  string `json:"dataId,omitempty"`
	Content string `json:"content"`
}

func (s *Service) moderateAliyun(ctx context.Context, cfg Config, input string, images []ModerationImage) (Decision, int64, error) {
	cfg = NormalizeConfig(cfg)
	if cfg.AliyunAccessKeyID == "" || cfg.AliyunAccessKeySecret == "" {
		return Decision{}, 0, newRiskControlConfigError("aliyun moderation access key is not configured")
	}
	if cfg.AliyunTextService == "" {
		return Decision{}, 0, newRiskControlConfigError("aliyun moderation text service is not configured")
	}
	if len(images) > 0 {
		return Decision{}, 0, errors.New("aliyun image moderation requires public image URLs")
	}
	input = normalizeInputText(input)
	if input == "" {
		return Decision{Allowed: true, Action: ActionAllow, CategoryScores: map[string]float64{}}, 0, nil
	}
	client, err := newAliyunModerationClient(cfg)
	if err != nil {
		return Decision{}, 0, wrapRiskControlConfigError(err)
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	startedAt := time.Now()
	resultCh := make(chan struct {
		decision Decision
		err      error
	}, 1)
	go func() {
		decision, err := moderateAliyunTextChunks(client, cfg, input)
		resultCh <- struct {
			decision Decision
			err      error
		}{decision: decision, err: err}
	}()
	select {
	case <-reqCtx.Done():
		return Decision{}, time.Since(startedAt).Milliseconds(), reqCtx.Err()
	case result := <-resultCh:
		return result.decision, time.Since(startedAt).Milliseconds(), result.err
	}
}

func newAliyunModerationClient(cfg Config) (*green.Client, error) {
	openapiCfg := &openapi.Config{
		AccessKeyId:     dara.String(cfg.AliyunAccessKeyID),
		AccessKeySecret: dara.String(cfg.AliyunAccessKeySecret),
		RegionId:        dara.String(cfg.AliyunRegionID),
	}
	if cfg.AliyunEndpoint != "" {
		openapiCfg.Endpoint = dara.String(cfg.AliyunEndpoint)
	}
	return green.NewClient(openapiCfg)
}

func moderateAliyunTextChunks(client *green.Client, cfg Config, input string) (Decision, error) {
	timeoutMS := cfg.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = DefaultConfig().TimeoutMS
	}
	runtime := &dara.RuntimeOptions{
		ReadTimeout:    dara.Int(timeoutMS),
		ConnectTimeout: dara.Int(timeoutMS),
	}
	for _, chunk := range aliyunContentChunks(input) {
		payload, err := json.Marshal(aliyunTextModerationPayload{Content: chunk})
		if err != nil {
			return Decision{}, err
		}
		resp, err := client.TextModerationPlusWithOptions(&green.TextModerationPlusRequest{
			Service:           dara.String(cfg.AliyunTextService),
			ServiceParameters: dara.String(string(payload)),
		}, runtime)
		if err != nil {
			return Decision{}, err
		}
		decision, err := decisionFromAliyunTextResponse(resp, cfg.AliyunBlockRiskLevel)
		if err != nil {
			return Decision{}, err
		}
		if decision.Flagged {
			return decision, nil
		}
	}
	return Decision{
		Allowed:         true,
		Action:          ActionAllow,
		Flagged:         false,
		CategoryScores:  map[string]float64{},
		Provider:        ProviderAliyun,
		RiskLevel:       "none",
		ProviderReason:  "aliyun risk level: none",
		HighestCategory: "",
		HighestScore:    0,
	}, nil
}

func decisionFromAliyunTextResponse(resp *green.TextModerationPlusResponse, blockRiskLevel string) (Decision, error) {
	if resp == nil || resp.Body == nil {
		return Decision{}, errors.New("aliyun moderation returned empty response")
	}
	code := int32(200)
	if resp.Body.Code != nil {
		code = *resp.Body.Code
	}
	if code != 200 {
		message := strings.TrimSpace(dara.StringValue(resp.Body.Message))
		if message == "" {
			message = fmt.Sprintf("code %d", code)
		}
		return Decision{}, fmt.Errorf("aliyun moderation failed: %s", message)
	}
	data := resp.Body.Data
	if data == nil {
		return Decision{
			Allowed:        true,
			Action:         ActionAllow,
			CategoryScores: map[string]float64{},
			Provider:       ProviderAliyun,
			RiskLevel:      "none",
		}, nil
	}
	riskLevel := normalizeAliyunRiskLevel(dara.StringValue(data.RiskLevel))
	score := aliyunScore(data)
	category := aliyunHighestCategory(data)
	if category == "" {
		category = riskLevel
	}
	scores := map[string]float64{}
	if category != "" {
		scores["aliyun:"+category] = score
	}
	flagged := aliyunRiskRank(riskLevel) >= aliyunRiskRank(aliyunBlockRiskLevel(blockRiskLevel))
	reason := aliyunReason(riskLevel, data)
	return Decision{
		Allowed:         true,
		Action:          ActionAllow,
		Flagged:         flagged,
		HighestCategory: "aliyun:" + category,
		HighestScore:    score,
		CategoryScores:  scores,
		Provider:        ProviderAliyun,
		RiskLevel:       riskLevel,
		ProviderReason:  reason,
	}, nil
}

func aliyunContentChunks(content string) []string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]string, 0, len(runes)/aliyunMaxContentRunes+1)
	for len(runes) > 0 {
		end := aliyunMaxContentRunes
		if len(runes) < end {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

func normalizeAliyunRiskLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "none", "":
		return "none"
	case RiskLevelLow:
		return RiskLevelLow
	case RiskLevelMedium:
		return RiskLevelMedium
	case RiskLevelHigh:
		return RiskLevelHigh
	default:
		return RiskLevelHigh
	}
}

func aliyunBlockRiskLevel(value string) string {
	level := normalizeRiskLevel(value)
	if level == "" {
		return RiskLevelLow
	}
	return level
}

func aliyunRiskRank(value string) int {
	switch normalizeAliyunRiskLevel(value) {
	case RiskLevelLow:
		return 1
	case RiskLevelMedium:
		return 2
	case RiskLevelHigh:
		return 3
	default:
		return 0
	}
}

func aliyunScore(data *green.TextModerationPlusResponseBodyData) float64 {
	if data == nil || data.Score == nil {
		return 0
	}
	score := float64(*data.Score)
	if score > 1 {
		score = score / 100
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func aliyunHighestCategory(data *green.TextModerationPlusResponseBodyData) string {
	if data == nil {
		return ""
	}
	for _, item := range data.Result {
		if item != nil && strings.TrimSpace(dara.StringValue(item.Label)) != "" {
			return strings.TrimSpace(dara.StringValue(item.Label))
		}
	}
	for _, item := range data.AttackResult {
		if item != nil && strings.TrimSpace(dara.StringValue(item.Label)) != "" {
			return strings.TrimSpace(dara.StringValue(item.Label))
		}
	}
	for _, item := range data.SensitiveResult {
		if item != nil && strings.TrimSpace(dara.StringValue(item.Label)) != "" {
			return strings.TrimSpace(dara.StringValue(item.Label))
		}
	}
	return normalizeAliyunRiskLevel(dara.StringValue(data.RiskLevel))
}

func aliyunReason(riskLevel string, data *green.TextModerationPlusResponseBodyData) string {
	labels := make([]string, 0, 4)
	if data != nil {
		for _, item := range data.Result {
			if item != nil {
				labels = appendAliyunLabel(labels, dara.StringValue(item.Label))
			}
		}
		for _, item := range data.AttackResult {
			if item != nil {
				labels = appendAliyunLabel(labels, dara.StringValue(item.Label))
			}
		}
		for _, item := range data.SensitiveResult {
			if item != nil {
				labels = appendAliyunLabel(labels, dara.StringValue(item.Label))
			}
		}
	}
	if len(labels) == 0 {
		return "aliyun risk level: " + riskLevel
	}
	return "aliyun risk level: " + riskLevel + "; labels: " + strings.Join(labels, ", ")
}

func appendAliyunLabel(labels []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return labels
	}
	for _, existing := range labels {
		if existing == value {
			return labels
		}
	}
	return append(labels, value)
}
