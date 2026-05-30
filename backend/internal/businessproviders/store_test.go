package businessproviders

import (
	"context"
	"os"
	"strings"
	"testing"

	"imagestudio/internal/config"
)

func newTestStore(t *testing.T) (*Store, *config.Config) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	cfg := config.New(t.TempDir())
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = dsn
	cfg.Database.MaxOpenConns = 4
	cfg.Database.MaxIdleConns = 2
	cfg.Database.ConnMaxLifetimeSeconds = 60
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("open provider store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := clearTestProviders(context.Background(), store); err != nil {
		t.Fatalf("clear provider store: %v", err)
	}
	return store, cfg
}

func clearTestProviders(ctx context.Context, store *Store) error {
	_, err := store.db.ExecContext(ctx, `TRUNCATE business_provider_members, business_provider_groups, business_api_providers RESTART IDENTITY CASCADE`)
	return err
}

func TestStoreCreatesAndSelectsDefaultProvider(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	first, err := store.Create(ctx, MutationInput{
		Name:         "first",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://first.example/v1",
		APIKey:       "first-key",
		DefaultModel: "gpt-image-2",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create first provider: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("first enabled provider should become default")
	}

	second, err := store.Create(ctx, MutationInput{
		Name:         "second",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://second.example/v1",
		APIKey:       "second-key",
		DefaultModel: "gpt-image-2",
		Enabled:      true,
		IsDefault:    true,
	})
	if err != nil {
		t.Fatalf("create second provider: %v", err)
	}

	defaultProvider, ok, err := store.DefaultForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("get default provider: %v", err)
	}
	if !ok {
		t.Fatal("default provider not found")
	}
	if defaultProvider.ID != second.ID {
		t.Fatalf("default provider = %q, want %q", defaultProvider.ID, second.ID)
	}

	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defaultCount := 0
	for _, item := range items {
		if item.Platform == PlatformGPTImage && item.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("default provider count = %d, want 1", defaultCount)
	}
}

func TestEnsureConfigProviderBackfillsLegacyAPIAccess(t *testing.T) {
	ctx := context.Background()
	store, cfg := newTestStore(t)
	defer store.Close()

	cfg.APIAccess.Platform = PlatformGPTImage
	cfg.APIAccess.BaseURL = "https://legacy.example/v1"
	cfg.APIAccess.APIKey = "legacy-key"

	if err := store.EnsureConfigProvider(ctx, cfg); err != nil {
		t.Fatalf("ensure config provider: %v", err)
	}
	defaultProvider, ok, err := store.DefaultForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("get default provider: %v", err)
	}
	if !ok {
		t.Fatal("default provider not found")
	}
	if defaultProvider.BaseURL != "https://legacy.example/v1" || defaultProvider.APIKey != "legacy-key" {
		t.Fatalf("default provider = %#v", defaultProvider)
	}
}

func TestGeminiBananaProviderUsesPlatformDefaultModel(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	item, err := store.Create(ctx, MutationInput{
		Name:     "banana",
		Platform: PlatformGeminiBanana,
		BaseURL:  "https://banana.example",
		APIKey:   "banana-key",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create gemini provider: %v", err)
	}
	if item.DefaultModel != "gemini-2.5-flash-image" {
		t.Fatalf("default model = %q, want gemini-2.5-flash-image", item.DefaultModel)
	}
}

func TestStoreSelectsProviderPoolMemberByGroupAndMemberPriority(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	_, err := store.CreateGroup(ctx, GroupInput{
		Name:      "secondary",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		Priority:  10,
		IsDefault: false,
	})
	if err != nil {
		t.Fatalf("create secondary group: %v", err)
	}
	defaultGroup, err := store.CreateGroup(ctx, GroupInput{
		Name:      "default",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		Priority:  100,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create default group: %v", err)
	}
	slow, err := store.CreateMember(ctx, MemberInput{
		GroupID:      defaultGroup.ID,
		Name:         "slow",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://slow.example/v1",
		APIKey:       "slow-key",
		DefaultModel: "gpt-image-slow",
		Enabled:      true,
		Priority:     50,
	})
	if err != nil {
		t.Fatalf("create slow member: %v", err)
	}
	fast, err := store.CreateMember(ctx, MemberInput{
		GroupID:      defaultGroup.ID,
		Name:         "fast",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://fast.example/v1",
		APIKey:       "fast-key",
		DefaultModel: "gpt-image-fast",
		Enabled:      true,
		Priority:     10,
	})
	if err != nil {
		t.Fatalf("create fast member: %v", err)
	}

	selection, ok, err := store.SelectDefaultMemberForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("select pool member: %v", err)
	}
	if !ok {
		t.Fatal("pool member not found")
	}
	if selection.Group.ID != defaultGroup.ID || selection.Member.ID != fast.ID {
		t.Fatalf("selection = group %q member %q, want %q/%q", selection.Group.ID, selection.Member.ID, defaultGroup.ID, fast.ID)
	}
	if selection.Member.ID == slow.ID {
		t.Fatal("lower priority member should be selected first")
	}
}

func TestStoreSkipsCoolingPoolMember(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	group, err := store.CreateGroup(ctx, GroupInput{
		Name:      "pool",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	cooling, err := store.CreateMember(ctx, MemberInput{
		GroupID:      group.ID,
		Name:         "cooling",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://cooling.example/v1",
		APIKey:       "cooling-key",
		DefaultModel: "gpt-image-cooling",
		Enabled:      true,
		Priority:     1,
	})
	if err != nil {
		t.Fatalf("create cooling member: %v", err)
	}
	ready, err := store.CreateMember(ctx, MemberInput{
		GroupID:      group.ID,
		Name:         "ready",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://ready.example/v1",
		APIKey:       "ready-key",
		DefaultModel: "gpt-image-ready",
		Enabled:      true,
		Priority:     2,
	})
	if err != nil {
		t.Fatalf("create ready member: %v", err)
	}
	if err := store.ReportMemberFailure(ctx, cooling.ID, MemberFailure{
		Status:          MemberStatusLimited,
		CooldownSeconds: 600,
		ErrorMessage:    "rate limited",
	}); err != nil {
		t.Fatalf("report failure: %v", err)
	}

	selection, ok, err := store.SelectDefaultMemberForPlatform(ctx, PlatformGPTImage)
	if err != nil {
		t.Fatalf("select pool member: %v", err)
	}
	if !ok {
		t.Fatal("pool member not found")
	}
	if selection.Member.ID != ready.ID {
		t.Fatalf("selected member = %q, want ready member %q", selection.Member.ID, ready.ID)
	}
}

func TestStoreSelectsTaggedPoolBeforeFallback(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	fallbackGroup, err := store.CreateGroup(ctx, GroupInput{
		Name:      "fallback",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
		Priority:  1,
		MatchMode: GroupMatchFallback,
	})
	if err != nil {
		t.Fatalf("create fallback group: %v", err)
	}
	taggedGroup, err := store.CreateGroup(ctx, GroupInput{
		Name:      "high",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		Priority:  100,
		Tags:      "quality:high,mode:generate",
		MatchMode: GroupMatchAny,
	})
	if err != nil {
		t.Fatalf("create tagged group: %v", err)
	}
	fallbackMember, err := store.CreateMember(ctx, MemberInput{
		GroupID:      fallbackGroup.ID,
		Name:         "fallback-member",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://fallback.example/v1",
		APIKey:       "fallback-key",
		DefaultModel: "gpt-image-fallback",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create fallback member: %v", err)
	}
	taggedMember, err := store.CreateMember(ctx, MemberInput{
		GroupID:      taggedGroup.ID,
		Name:         "high-member",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://high.example/v1",
		APIKey:       "high-key",
		DefaultModel: "gpt-image-high",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create tagged member: %v", err)
	}

	selection, ok, err := store.SelectMember(ctx, PoolSelectionPolicy{
		Platform: PlatformGPTImage,
		Tags:     []string{"quality:high"},
	})
	if err != nil {
		t.Fatalf("select tagged member: %v", err)
	}
	if !ok {
		t.Fatal("tagged selection not found")
	}
	if selection.Group.ID != taggedGroup.ID || selection.Member.ID != taggedMember.ID {
		t.Fatalf("selection = %q/%q, want %q/%q", selection.Group.ID, selection.Member.ID, taggedGroup.ID, taggedMember.ID)
	}

	selection, ok, err = store.SelectMember(ctx, PoolSelectionPolicy{
		Platform: PlatformGPTImage,
		Tags:     []string{"quality:low"},
	})
	if err != nil {
		t.Fatalf("select fallback member: %v", err)
	}
	if !ok {
		t.Fatal("fallback selection not found")
	}
	if selection.Group.ID != fallbackGroup.ID || selection.Member.ID != fallbackMember.ID {
		t.Fatalf("fallback selection = %q/%q, want %q/%q", selection.Group.ID, selection.Member.ID, fallbackGroup.ID, fallbackMember.ID)
	}
}

func TestStoreFallsBackWhenTaggedPoolHasNoAvailableMember(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	fallbackGroup, err := store.CreateGroup(ctx, GroupInput{
		Name:      "fallback",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
		Priority:  1,
		MatchMode: GroupMatchFallback,
	})
	if err != nil {
		t.Fatalf("create fallback group: %v", err)
	}
	taggedGroup, err := store.CreateGroup(ctx, GroupInput{
		Name:      "meridian",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		Priority:  1,
		Tags:      "tier:meridian",
		MatchMode: GroupMatchAll,
	})
	if err != nil {
		t.Fatalf("create tagged group: %v", err)
	}
	fallbackMember, err := store.CreateMember(ctx, MemberInput{
		GroupID:      fallbackGroup.ID,
		Name:         "fallback-member",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://fallback.example/v1",
		APIKey:       "fallback-key",
		DefaultModel: "gpt-image-fallback",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("create fallback member: %v", err)
	}
	_, err = store.CreateMember(ctx, MemberInput{
		GroupID:      taggedGroup.ID,
		Name:         "unavailable-member",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://tagged.example/v1",
		APIKey:       "tagged-key",
		DefaultModel: "gpt-image-tagged",
		Enabled:      true,
		Status:       MemberStatusUnavailable,
	})
	if err != nil {
		t.Fatalf("create tagged member: %v", err)
	}

	selection, ok, err := store.SelectMember(ctx, PoolSelectionPolicy{
		Platform: PlatformGPTImage,
		Tags:     []string{"tier:meridian"},
	})
	if err != nil {
		t.Fatalf("select fallback after tagged unavailable: %v", err)
	}
	if !ok {
		t.Fatal("fallback selection not found")
	}
	if selection.Group.ID != fallbackGroup.ID || selection.Member.ID != fallbackMember.ID {
		t.Fatalf("selection = %q/%q, want %q/%q", selection.Group.ID, selection.Member.ID, fallbackGroup.ID, fallbackMember.ID)
	}
	if selection.Strategy != SelectionStrategyTaggedFallback {
		t.Fatalf("strategy = %q, want %q", selection.Strategy, SelectionStrategyTaggedFallback)
	}
}

func TestStoreReportsProviderPoolMemberHealth(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	group, err := store.CreateGroup(ctx, GroupInput{
		Name:      "pool",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	member, err := store.CreateMember(ctx, MemberInput{
		GroupID:  group.ID,
		Name:     "member",
		Platform: PlatformGPTImage,
		BaseURL:  "https://member.example/v1",
		APIKey:   "member-key",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	if err := store.ReportMemberFailure(ctx, member.ID, MemberFailure{
		Status:          MemberStatusLimited,
		CooldownSeconds: 60,
		ErrorMessage:    "temporary failure",
	}); err != nil {
		t.Fatalf("report failure: %v", err)
	}
	failed, ok, err := store.GetMember(ctx, member.ID)
	if err != nil || !ok {
		t.Fatalf("get failed member ok=%v err=%v", ok, err)
	}
	if failed.FailCount != 1 || failed.Status != MemberStatusLimited || failed.LastError != "temporary failure" || failed.CooldownUntil == "" {
		t.Fatalf("failed member = %#v", failed)
	}

	if err := store.ReportMemberSuccess(ctx, member.ID); err != nil {
		t.Fatalf("report success: %v", err)
	}
	succeeded, ok, err := store.GetMember(ctx, member.ID)
	if err != nil || !ok {
		t.Fatalf("get succeeded member ok=%v err=%v", ok, err)
	}
	if succeeded.SuccessCount != 1 || succeeded.Status != MemberStatusActive || succeeded.CooldownUntil != "" || succeeded.LastError != "" {
		t.Fatalf("succeeded member = %#v", succeeded)
	}
}

func TestStoreAutoDisablesAndRecoversFailingPoolMember(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	group, err := store.CreateGroup(ctx, GroupInput{
		Name:      "pool",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	member, err := store.CreateMember(ctx, MemberInput{
		GroupID:          group.ID,
		Name:             "fragile",
		Platform:         PlatformGPTImage,
		BaseURL:          "https://fragile.example/v1",
		APIKey:           "fragile-key",
		Enabled:          true,
		FailureThreshold: 2,
		CooldownSeconds:  1,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := store.ReportMemberFailure(ctx, member.ID, MemberFailure{
			Status:       MemberStatusLimited,
			ErrorMessage: "temporary failure",
		}); err != nil {
			t.Fatalf("report failure %d: %v", i+1, err)
		}
	}
	failed, ok, err := store.GetMember(ctx, member.ID)
	if err != nil || !ok {
		t.Fatalf("get failed member ok=%v err=%v", ok, err)
	}
	if failed.Status != MemberStatusUnavailable || failed.ConsecutiveFailures != 2 || failed.CooldownUntil != "" {
		t.Fatalf("failed member = %#v", failed)
	}

	recovered, ok, err := store.RecoverMember(ctx, member.ID)
	if err != nil || !ok {
		t.Fatalf("recover member ok=%v err=%v", ok, err)
	}
	if recovered.Status != MemberStatusActive || !recovered.Enabled || recovered.ConsecutiveFailures != 0 || recovered.LastError != "" {
		t.Fatalf("recovered member = %#v", recovered)
	}
}

func TestStoreManagesProviderPools(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	group, err := store.CreateGroup(ctx, GroupInput{
		Name:        "primary pool",
		Platform:    PlatformGPTImage,
		Description: "default lane",
		Enabled:     true,
		IsDefault:   true,
		Priority:    20,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	member, err := store.CreateMember(ctx, MemberInput{
		GroupID:      group.ID,
		Name:         "member one",
		BaseURL:      "https://member-one.example/v1",
		APIKey:       "member-one-key",
		DefaultModel: "gpt-image-one",
		Enabled:      true,
		Priority:     30,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	pools, err := store.ListPools(ctx)
	if err != nil {
		t.Fatalf("list pools: %v", err)
	}
	if len(pools) != 1 || pools[0].ID != group.ID || len(pools[0].Members) != 1 || pools[0].Members[0].ID != member.ID {
		t.Fatalf("pools = %#v", pools)
	}
	if pools[0].Members[0].Platform != PlatformGPTImage {
		t.Fatalf("member platform = %q, want group platform", pools[0].Members[0].Platform)
	}

	updatedGroup, found, err := store.UpdateGroup(ctx, group.ID, GroupInput{
		Name:        "updated pool",
		Platform:    PlatformGPTImage,
		Description: "updated",
		Enabled:     false,
		Priority:    5,
	})
	if err != nil || !found {
		t.Fatalf("update group found=%v err=%v", found, err)
	}
	if updatedGroup.Name != "updated pool" || updatedGroup.Enabled || updatedGroup.Priority != 5 {
		t.Fatalf("updated group = %#v", updatedGroup)
	}

	updatedMember, found, err := store.UpdateMember(ctx, member.ID, MemberInput{
		GroupID:      group.ID,
		Name:         "member two",
		Platform:     PlatformGPTImage,
		BaseURL:      "https://member-two.example/v1",
		APIKey:       "member-two-key",
		DefaultModel: "gpt-image-two",
		Enabled:      false,
		Priority:     7,
		Status:       MemberStatusUnavailable,
	})
	if err != nil || !found {
		t.Fatalf("update member found=%v err=%v", found, err)
	}
	if updatedMember.Name != "member two" || updatedMember.Enabled || updatedMember.Priority != 7 || updatedMember.Status != MemberStatusUnavailable {
		t.Fatalf("updated member = %#v", updatedMember)
	}

	deletedMember, err := store.DeleteMember(ctx, member.ID)
	if err != nil || !deletedMember {
		t.Fatalf("delete member deleted=%v err=%v", deletedMember, err)
	}
	deletedGroup, err := store.DeleteGroup(ctx, group.ID)
	if err != nil || !deletedGroup {
		t.Fatalf("delete group deleted=%v err=%v", deletedGroup, err)
	}
	pools, err = store.ListPools(ctx)
	if err != nil {
		t.Fatalf("list empty pools: %v", err)
	}
	if len(pools) != 0 {
		t.Fatalf("pools after delete = %#v", pools)
	}
}

func TestStoreRejectsGroupPlatformChangeWithMembers(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t)
	defer store.Close()

	group, err := store.CreateGroup(ctx, GroupInput{
		Name:      "pool",
		Platform:  PlatformGPTImage,
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := store.CreateMember(ctx, MemberInput{
		GroupID: group.ID,
		Name:    "member",
		BaseURL: "https://member.example/v1",
		APIKey:  "member-key",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, _, err := store.UpdateGroup(ctx, group.ID, GroupInput{
		Name:      "pool",
		Platform:  PlatformGeminiBanana,
		Enabled:   true,
		IsDefault: true,
	}); err == nil {
		t.Fatal("expected platform change with members to fail")
	}
}
