package riskcontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	maxInputRunes   = 12000
	maxExcerptRunes = 240
)

type CheckInput struct {
	UserID         string
	JobID          string
	ConversationID string
	TurnID         string
	Platform       string
	Model          string
	Prompt         string
	Images         []ModerationImage
}

type ModerationImage struct {
	MimeType string
	Data     []byte
}

type Decision struct {
	Allowed           bool               `json:"allowed"`
	Action            string             `json:"action"`
	Flagged           bool               `json:"flagged"`
	HighestCategory   string             `json:"highestCategory"`
	HighestScore      float64            `json:"highestScore"`
	CategoryScores    map[string]float64 `json:"categoryScores"`
	Message           string             `json:"message,omitempty"`
	Error             string             `json:"error,omitempty"`
	InternalErrorCode string             `json:"-"`
}

type TestResult struct {
	Decision  Decision `json:"decision"`
	LatencyMS int64    `json:"latencyMs"`
}

type TestConfigOverride struct {
	Mode         *string
	BaseURL      *string
	APIKey       *string
	ClearAPIKey  bool
	Model        *string
	TimeoutMS    *int
	BlockMessage *string
	Thresholds   *map[string]float64
}

type Service struct {
	store      *Store
	httpClient *http.Client
}

var errRiskControlConfig = errors.New("risk control config error")

func newRiskControlConfigError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "invalid risk control configuration"
	}
	return fmt.Errorf("%w: %s", errRiskControlConfig, message)
}

func wrapRiskControlConfigError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errRiskControlConfig, err)
}

func ErrorDecision(err error) Decision {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return Decision{
		Allowed:           false,
		Action:            ActionError,
		CategoryScores:    map[string]float64{},
		Error:             message,
		InternalErrorCode: decisionInternalErrorCode(err),
	}
}

func NewService(store *Store) *Service {
	return &Service{
		store:      store,
		httpClient: &http.Client{},
	}
}

func (s *Service) Check(ctx context.Context, input CheckInput) (Decision, error) {
	allow := Decision{Allowed: true, Action: ActionAllow, CategoryScores: map[string]float64{}}
	if s == nil || s.store == nil {
		return allow, nil
	}
	prompt := normalizeInputText(input.Prompt)
	images := normalizeImages(input.Images)
	if prompt == "" && len(images) == 0 {
		return allow, nil
	}
	cfg, err := s.store.GetConfig(ctx)
	if err != nil {
		decision := ErrorDecision(err)
		_, _ = s.store.InsertLog(context.Background(), LogInput{
			UserID:         input.UserID,
			JobID:          input.JobID,
			ConversationID: input.ConversationID,
			TurnID:         input.TurnID,
			Platform:       input.Platform,
			Model:          input.Model,
			Mode:           ModeObserve,
			Action:         ActionError,
			InputExcerpt:   inputExcerpt(prompt, len(images)),
			Error:          decision.Error,
		})
		return decision, nil
	}
	cfg = NormalizeConfig(cfg)
	if !cfg.Enabled {
		return allow, nil
	}
	startedAt := time.Now()
	decision, latencyMS, err := s.moderate(ctx, cfg, prompt, images)
	logInput := LogInput{
		UserID:         input.UserID,
		JobID:          input.JobID,
		ConversationID: input.ConversationID,
		TurnID:         input.TurnID,
		Platform:       input.Platform,
		Model:          input.Model,
		Mode:           cfg.Mode,
		InputExcerpt:   inputExcerpt(prompt, len(images)),
		LatencyMS:      latencyMS,
	}
	if latencyMS == 0 {
		logInput.LatencyMS = time.Since(startedAt).Milliseconds()
	}
	if err != nil {
		decision = ErrorDecision(err)
		logInput.Action = ActionError
		logInput.Error = err.Error()
		_, _ = s.store.InsertLog(context.Background(), logInput)
		return decision, nil
	}
	decision.Allowed = true
	decision.Action = ActionAllow
	if decision.Flagged && cfg.Mode == ModePreBlock {
		decision.Allowed = false
		decision.Action = ActionBlock
		decision.Message = cfg.BlockMessage
	}
	logInput.Action = decision.Action
	logInput.Flagged = decision.Flagged
	logInput.HighestCategory = decision.HighestCategory
	logInput.HighestScore = decision.HighestScore
	logInput.CategoryScores = decision.CategoryScores
	if decision.Flagged || cfg.RecordNonHits {
		_, _ = s.store.InsertLog(context.Background(), logInput)
	}
	return decision, nil
}

func (s *Service) Test(ctx context.Context, input string, override TestConfigOverride) (TestResult, error) {
	if s == nil || s.store == nil {
		return TestResult{}, errors.New("risk control service unavailable")
	}
	cfg, err := s.store.GetConfig(ctx)
	if err != nil {
		return TestResult{}, err
	}
	cfg = applyTestOverride(cfg, override)
	prompt := normalizeInputText(input)
	if prompt == "" {
		return TestResult{}, errors.New("prompt is required")
	}
	decision, latencyMS, err := s.moderate(ctx, cfg, prompt, nil)
	if err != nil {
		return TestResult{}, err
	}
	decision.Allowed = true
	decision.Action = ActionAllow
	if decision.Flagged && cfg.Mode == ModePreBlock {
		decision.Allowed = false
		decision.Action = ActionBlock
		decision.Message = cfg.BlockMessage
	}
	return TestResult{Decision: decision, LatencyMS: latencyMS}, nil
}

func applyTestOverride(cfg Config, override TestConfigOverride) Config {
	if override.BaseURL != nil {
		cfg.BaseURL = *override.BaseURL
	}
	if override.Mode != nil {
		cfg.Mode = *override.Mode
	}
	if override.ClearAPIKey {
		cfg.APIKey = ""
	} else if override.APIKey != nil && strings.TrimSpace(*override.APIKey) != "" {
		cfg.APIKey = *override.APIKey
	}
	if override.Model != nil {
		cfg.Model = *override.Model
	}
	if override.BlockMessage != nil {
		cfg.BlockMessage = *override.BlockMessage
	}
	if override.TimeoutMS != nil {
		cfg.TimeoutMS = *override.TimeoutMS
	}
	if override.Thresholds != nil {
		cfg.Thresholds = *override.Thresholds
	}
	return NormalizeConfig(cfg)
}

func (s *Service) moderate(ctx context.Context, cfg Config, input string, images []ModerationImage) (Decision, int64, error) {
	switch NormalizeConfig(cfg).Provider {
	case ProviderOpenAI:
		return s.moderateOpenAI(ctx, cfg, input, images)
	default:
		return Decision{}, 0, newRiskControlConfigError("unsupported risk control provider")
	}
}

func decisionInternalErrorCode(err error) string {
	if errors.Is(err, errRiskControlConfig) {
		return DecisionErrorConfig
	}
	return DecisionErrorUnavailable
}

func decisionFromScores(flagged bool, scores map[string]float64, thresholds map[string]float64) Decision {
	highestCategory := ""
	highestScore := 0.0
	for category, score := range scores {
		if highestCategory == "" || score > highestScore {
			highestCategory = category
			highestScore = score
		}
		if threshold, ok := thresholds[category]; ok && score >= threshold {
			flagged = true
		}
	}
	return Decision{
		Allowed:         true,
		Action:          ActionAllow,
		Flagged:         flagged,
		HighestCategory: highestCategory,
		HighestScore:    highestScore,
		CategoryScores:  copyScores(scores),
	}
}

func normalizeInputText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxInputRunes {
		return string(runes[:maxInputRunes])
	}
	return value
}

func excerpt(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxExcerptRunes {
		return string(runes)
	}
	return string(runes[:maxExcerptRunes]) + "..."
}

func inputExcerpt(prompt string, imageCount int) string {
	text := excerpt(prompt)
	if imageCount <= 0 {
		return text
	}
	imageText := fmt.Sprintf("[images:%d]", imageCount)
	if text == "" {
		return imageText
	}
	return text + " " + imageText
}

func normalizeImages(images []ModerationImage) []ModerationImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]ModerationImage, 0, len(images))
	for _, image := range images {
		if len(image.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(strings.ToLower(image.MimeType))
		if mimeType == "" {
			mimeType = http.DetectContentType(image.Data)
		}
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}
		out = append(out, ModerationImage{
			MimeType: mimeType,
			Data:     append([]byte(nil), image.Data...),
		})
	}
	return out
}
