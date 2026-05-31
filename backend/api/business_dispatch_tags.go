package api

import (
	"context"
	"strings"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesspayments"
	"imagestudio/internal/businesssettings"
)

const (
	defaultSubscriptionDispatchTag = "tier:free"
	defaultWalletDispatchTag       = "wallet:none"
)

func (s *Server) enrichProviderImageDispatchTags(ctx context.Context, userID string, payload map[string]any, metadata providerImageGenerateMetadata, trustedPayload bool) providerImageGenerateMetadata {
	if payload == nil {
		return metadata
	}
	requestTags := []string(nil)
	if trustedPayload {
		requestTags = providerDispatchTagStrings(payload["requestDispatchTags"])
	}
	if len(requestTags) == 0 {
		requestTags = metadata.DispatchTags
	}
	requestTags = filterProtectedProviderDispatchTags(requestTags)
	userTags := []string(nil)
	if trustedPayload {
		userTags = filterUserProviderDispatchTags(providerDispatchTagStrings(payload["userDispatchTags"]))
	}
	if len(userTags) == 0 {
		userTags = s.businessUserDispatchTags(ctx, userID)
	}
	metadata.DispatchTags = mergeProviderDispatchTags(requestTags, userTags)
	persistProviderDispatchTagsInPayload(payload, requestTags, userTags, metadata.DispatchTags)
	return metadata
}

func (s *Server) businessUserDispatchTags(ctx context.Context, userID string) []string {
	settings := s.businessSystemSettingsForContext(ctx)
	return mergeProviderDispatchTags(
		[]string{s.businessUserRoleDispatchTag(ctx, userID)},
		[]string{s.businessSubscriptionLevelDispatchTag(ctx, userID, settings)},
		[]string{s.businessWalletLevelDispatchTag(ctx, userID, settings)},
	)
}

func (s *Server) businessUserRoleDispatchTag(ctx context.Context, userID string) string {
	role := authRoleUser
	if store, err := s.newBusinessAuthStore(); err == nil {
		defer store.Close()
		if user, ok, getErr := store.GetUserByID(ctx, strings.TrimSpace(userID)); getErr == nil && ok && strings.TrimSpace(user.Role) != "" {
			role = user.Role
		}
	}
	return "role:" + normalizeDispatchTagPart(role, authRoleUser)
}

func (s *Server) businessSubscriptionLevelDispatchTag(ctx context.Context, userID string, settings businesssettings.Settings) string {
	fallback := fallbackBillingLevelTag(settings.Billing.SubscriptionLevels, defaultSubscriptionDispatchTag)
	if override := s.businessUserBillingLevelOverrideTag(ctx, userID, settings.Billing.SubscriptionLevels, "tier:", func(user businessauth.User) string {
		return user.SubscriptionLevelTag
	}); override != "" {
		return override
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		return fallback
	}
	defer store.Close()
	subscription, err := store.GetCurrentSubscription(ctx, strings.TrimSpace(userID))
	if err != nil || !subscription.Active || subscription.Status != businesspayments.SubscriptionStatusActive || strings.TrimSpace(subscription.PackageID) == "" {
		return fallback
	}
	pkg, ok, err := store.GetPackage(ctx, subscription.PackageID, true)
	if err != nil || !ok {
		return fallback
	}
	return selectConfiguredBillingLevelTag([]string{pkg.LevelTag}, settings.Billing.SubscriptionLevels, fallback)
}

func (s *Server) businessWalletLevelDispatchTag(ctx context.Context, userID string, settings businesssettings.Settings) string {
	fallback := fallbackBillingLevelTag(settings.Billing.WalletLevels, defaultWalletDispatchTag)
	if override := s.businessUserBillingLevelOverrideTag(ctx, userID, settings.Billing.WalletLevels, "wallet:", func(user businessauth.User) string {
		return user.WalletLevelTag
	}); override != "" {
		return override
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		return fallback
	}
	defer store.Close()
	tags, err := store.UserPaidPackageLevelTags(ctx, strings.TrimSpace(userID), businesspayments.PackageTypeBalance)
	if err != nil {
		return fallback
	}
	return selectConfiguredBillingLevelTag(tags, settings.Billing.WalletLevels, fallback)
}

func (s *Server) businessUserBillingLevelOverrideTag(
	ctx context.Context,
	userID string,
	levels []businesssettings.BillingLevelSettings,
	requiredPrefix string,
	pick func(businessauth.User) string,
) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		return ""
	}
	defer store.Close()
	user, ok, err := store.GetUserByID(ctx, userID)
	if err != nil || !ok {
		return ""
	}
	return configuredBillingLevelTag(pick(user), levels, requiredPrefix)
}

func persistProviderDispatchTagsInPayload(payload map[string]any, requestTags []string, userTags []string, dispatchTags []string) {
	if payload == nil {
		return
	}
	setProviderDispatchTagPayload(payload, "requestDispatchTags", requestTags)
	setProviderDispatchTagPayload(payload, "userDispatchTags", userTags)
	setProviderDispatchTagPayload(payload, "dispatchTags", dispatchTags)
}

func setProviderDispatchTagPayload(payload map[string]any, key string, tags []string) {
	tags = mergeProviderDispatchTags(tags)
	if len(tags) == 0 {
		delete(payload, key)
		return
	}
	payload[key] = tags
}

func mergeProviderDispatchTags(parts ...[]string) []string {
	seen := map[string]struct{}{}
	tags := make([]string, 0)
	for _, values := range parts {
		for _, raw := range values {
			tag := normalizeProviderDispatchTag(raw)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags
}

func filterProtectedProviderDispatchTags(tags []string) []string {
	items := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := normalizeProviderDispatchTag(raw)
		if tag == "" || isUserProviderDispatchTag(tag) {
			continue
		}
		items = append(items, tag)
	}
	return mergeProviderDispatchTags(items)
}

func filterUserProviderDispatchTags(tags []string) []string {
	items := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := normalizeProviderDispatchTag(raw)
		if tag == "" || !isUserProviderDispatchTag(tag) {
			continue
		}
		items = append(items, tag)
	}
	return mergeProviderDispatchTags(items)
}

func isUserProviderDispatchTag(tag string) bool {
	tag = normalizeProviderDispatchTag(tag)
	return strings.HasPrefix(tag, "tier:") || strings.HasPrefix(tag, "wallet:") || strings.HasPrefix(tag, "role:")
}

func selectConfiguredBillingLevelTag(candidateTags []string, levels []businesssettings.BillingLevelSettings, fallback string) string {
	fallback = normalizeProviderDispatchTag(fallback)
	if fallback == "" {
		fallback = defaultSubscriptionDispatchTag
	}
	candidates := map[string]struct{}{}
	for _, raw := range candidateTags {
		tag := normalizeProviderDispatchTag(raw)
		if tag != "" {
			candidates[tag] = struct{}{}
		}
	}
	bestTag := ""
	bestSort := -1
	for index, level := range levels {
		if !level.Enabled {
			continue
		}
		tag := normalizeProviderDispatchTag(level.Tag)
		if tag == "" {
			continue
		}
		if _, ok := candidates[tag]; !ok {
			continue
		}
		sortOrder := level.SortOrder
		if sortOrder < 0 {
			sortOrder = index * 10
		}
		if bestTag == "" || sortOrder >= bestSort {
			bestTag = tag
			bestSort = sortOrder
		}
	}
	if bestTag != "" {
		return bestTag
	}
	return fallback
}

func configuredBillingLevelTag(raw string, levels []businesssettings.BillingLevelSettings, requiredPrefix string) string {
	tag := normalizeProviderDispatchTag(raw)
	requiredPrefix = normalizeProviderDispatchTag(requiredPrefix)
	if tag == "" || (requiredPrefix != "" && !strings.HasPrefix(tag, requiredPrefix)) {
		return ""
	}
	for _, level := range levels {
		if !level.Enabled {
			continue
		}
		if normalizeProviderDispatchTag(level.Tag) == tag {
			return tag
		}
	}
	return ""
}

func fallbackBillingLevelTag(levels []businesssettings.BillingLevelSettings, fallback string) string {
	fallback = normalizeProviderDispatchTag(fallback)
	bestTag := ""
	bestSort := 0
	for index, level := range levels {
		if !level.Enabled {
			continue
		}
		tag := normalizeProviderDispatchTag(level.Tag)
		if tag == "" {
			continue
		}
		if fallback != "" && tag == fallback {
			return tag
		}
		sortOrder := level.SortOrder
		if sortOrder < 0 {
			sortOrder = index * 10
		}
		if bestTag == "" || sortOrder < bestSort {
			bestTag = tag
			bestSort = sortOrder
		}
	}
	if bestTag != "" {
		return bestTag
	}
	if fallback != "" {
		return fallback
	}
	return defaultSubscriptionDispatchTag
}

func normalizeProviderDispatchTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeDispatchTagPart(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return fallback
	}
	return result
}
