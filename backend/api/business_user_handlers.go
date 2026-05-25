package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"imagestudio/internal/businessauth"
	"imagestudio/internal/businesscredits"
	"imagestudio/internal/businessimage"
)

func paginationFromQuery(r *http.Request, prefix string, defaultPageSize int, maxPageSize int) (int, int, int) {
	pageKey := "page"
	pageSizeKey := "pageSize"
	if prefix != "" {
		pageKey = prefix + "Page"
		pageSizeKey = prefix + "PageSize"
	}
	page := intQueryValue(r, pageKey, 1)
	if page < 1 {
		page = 1
	}
	pageSize := intQueryValue(r, pageSizeKey, defaultPageSize)
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize, (page - 1) * pageSize
}

func intQueryValue(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func boolQueryValue(r *http.Request, key string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func usageRecordFilterFromQuery(r *http.Request, userID string) businessimage.UsageRecordFilter {
	from, to := usageTimeRangeFromQuery(r)
	return businessimage.UsageRecordFilter{
		UserID: strings.TrimSpace(firstNonEmpty(userID, r.URL.Query().Get("userId"))),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Model:  strings.TrimSpace(r.URL.Query().Get("model")),
		From:   from,
		To:     to,
	}
}

func usageTimeRangeFromQuery(r *http.Request) (string, string) {
	query := r.URL.Query()
	rawFrom := strings.TrimSpace(query.Get("from"))
	rawTo := strings.TrimSpace(query.Get("to"))
	timeRange := strings.ToLower(strings.TrimSpace(query.Get("timeRange")))
	location := timeRangeLocation(strings.TrimSpace(query.Get("timezone")))
	if timeRange == "" || timeRange == "custom" {
		return rawFrom, rawTo
	}
	now := time.Now().In(location)
	start, end := localDayBounds(now)
	switch timeRange {
	case "today":
	case "24h", "last24h":
		start = now.Add(-24 * time.Hour)
		end = now
	case "yesterday":
		day := now.AddDate(0, 0, -1)
		start, end = localDayBounds(day)
	case "last7", "7d":
		start = start.AddDate(0, 0, -6)
	case "last14", "14d":
		start = start.AddDate(0, 0, -13)
	case "last30", "30d":
		start = start.AddDate(0, 0, -29)
	case "month", "this_month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		end = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, location).Add(-time.Nanosecond)
	case "last_month":
		firstThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		start = firstThisMonth.AddDate(0, -1, 0)
		end = firstThisMonth.Add(-time.Nanosecond)
	default:
		return rawFrom, rawTo
	}
	return start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)
}

func timeRangeLocation(value string) *time.Location {
	if value == "" {
		return time.Local
	}
	location, err := time.LoadLocation(value)
	if err != nil {
		return time.Local
	}
	return location
}

func localDayBounds(value time.Time) (time.Time, time.Time) {
	location := value.Location()
	start := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
	return start, start.AddDate(0, 0, 1).Add(-time.Nanosecond)
}

type paginationMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type businessUserWithUsage struct {
	businessauth.User
	Usage  businessimage.UserUsage `json:"usage"`
	Credit businesscredits.Summary `json:"credit"`
}

type businessUsageRecordWithUser struct {
	businessimage.UsageRecord
	UID      int64  `json:"uid,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

var errBusinessUserNotDeleted = errors.New("business user is not deleted")

type businessMeResponse struct {
	User   businessauth.User       `json:"user"`
	Credit businesscredits.Summary `json:"credit"`
}

type businessCreditLedgerResponse struct {
	Items []businesscredits.LedgerEntry `json:"items"`
	Page  paginationMeta                `json:"page"`
}

type businessUserDetailResponse struct {
	User               businessauth.User             `json:"user"`
	Usage              businessimage.UserUsage       `json:"usage"`
	Credit             businesscredits.Summary       `json:"credit"`
	RecentUsage        []businessUsageRecordWithUser `json:"recentUsage"`
	RecentUsagePage    paginationMeta                `json:"recentUsagePage"`
	RecentAssets       []businessimage.Asset         `json:"recentAssets"`
	RecentAssetsPage   paginationMeta                `json:"recentAssetsPage"`
	RecentCreditLedger []businesscredits.LedgerEntry `json:"recentCreditLedger"`
	RecentLedgerPage   paginationMeta                `json:"recentLedgerPage"`
	ConversationCount  int64                         `json:"conversationCount"`
}

type businessDashboardSummary struct {
	UserCount          int64  `json:"userCount"`
	AdminCount         int64  `json:"adminCount"`
	ActiveUserCount    int64  `json:"activeUserCount"`
	DisabledUserCount  int64  `json:"disabledUserCount"`
	GenerationCount    int64  `json:"generationCount"`
	SuccessCount       int64  `json:"successCount"`
	FailedCount        int64  `json:"failedCount"`
	ImageCount         int64  `json:"imageCount"`
	StorageBytes       int64  `json:"storageBytes"`
	ConversationCount  int64  `json:"conversationCount"`
	CreditBalance      int64  `json:"creditBalance"`
	CreditSpent        int64  `json:"creditSpent"`
	LastGeneratedAt    string `json:"lastGeneratedAt,omitempty"`
	RecentUsageCount   int64  `json:"recentUsageCount"`
	RecentSuccessCount int64  `json:"recentSuccessCount"`
	RecentFailedCount  int64  `json:"recentFailedCount"`
	RecentImageCount   int64  `json:"recentImageCount"`
	RecentCreditsUsed  int64  `json:"recentCreditsUsed"`
}

type businessDashboardTopUser struct {
	businessauth.User
	Usage  businessimage.UserUsage `json:"usage"`
	Credit businesscredits.Summary `json:"credit"`
}

type businessDashboardResponse struct {
	Summary    businessDashboardSummary      `json:"summary"`
	Recent     []businessUsageRecordWithUser `json:"recent"`
	TopUsers   []businessDashboardTopUser    `json:"topUsers"`
	ModelUsage []businessimage.ModelUsage    `json:"modelUsage"`
	Page       paginationMeta                `json:"page"`
}

func (s *Server) handleGetBusinessDashboard(w http.ResponseWriter, r *http.Request) {
	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	users, err := userStore.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	from, to := usageTimeRangeFromQuery(r)
	usageFilter := businessimage.UsageSummaryFilter{From: from, To: to}
	usages, err := imageStore.UserUsageFiltered(r.Context(), usageFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	conversationCount, err := imageStore.ConversationTotalFiltered(r.Context(), usageFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	modelUsage, err := imageStore.ModelUsageFiltered(r.Context(), usageFilter, 8)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	credits, err := creditStore.Summaries(r.Context(), userIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	recent, recentTotal, err := imageStore.UsageRecordsPage(r.Context(), usageRecordFilterFromQuery(r, ""), 10, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	filteredUsageRecords, _, err := imageStore.UsageRecordsPage(r.Context(), businessimage.UsageRecordFilter{From: from, To: to}, 500, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	creditSpentByUserID := make(map[string]int64)
	for _, record := range filteredUsageRecords {
		creditSpentByUserID[record.UserID] += record.CreditsUsed
	}

	userByID := make(map[string]businessauth.User, len(users))
	summary := businessDashboardSummary{
		UserCount:         int64(len(users)),
		ConversationCount: conversationCount,
	}
	topUsers := make([]businessDashboardTopUser, 0, len(users))
	for _, user := range users {
		userByID[user.ID] = user
		if user.Role == businessauth.RoleAdmin {
			summary.AdminCount += 1
		}
		if user.Status == businessauth.StatusActive {
			summary.ActiveUserCount += 1
		}
		if user.Status == businessauth.StatusDisabled {
			summary.DisabledUserCount += 1
		}
		usage := usages[user.ID]
		usage.UserID = user.ID
		credit := credits[user.ID]
		credit.UserID = user.ID
		credit.Spent = creditSpentByUserID[user.ID]
		summary.GenerationCount += usage.GenerationCount
		summary.SuccessCount += usage.SuccessCount
		summary.FailedCount += usage.FailedCount
		summary.ImageCount += usage.ImageCount
		summary.StorageBytes += usage.StorageBytes
		summary.CreditBalance += credit.Balance
		summary.CreditSpent += credit.Spent
		if usage.LastGeneratedAt > summary.LastGeneratedAt {
			summary.LastGeneratedAt = usage.LastGeneratedAt
		}
		topUsers = append(topUsers, businessDashboardTopUser{
			User:   user,
			Usage:  usage,
			Credit: credit,
		})
	}

	sort.SliceStable(topUsers, func(i, j int) bool {
		if topUsers[i].Credit.Spent != topUsers[j].Credit.Spent {
			return topUsers[i].Credit.Spent > topUsers[j].Credit.Spent
		}
		if topUsers[i].Usage.GenerationCount != topUsers[j].Usage.GenerationCount {
			return topUsers[i].Usage.GenerationCount > topUsers[j].Usage.GenerationCount
		}
		return topUsers[i].Username < topUsers[j].Username
	})
	if len(topUsers) > 6 {
		topUsers = topUsers[:6]
	}

	recentWithUser := make([]businessUsageRecordWithUser, 0, len(recent))
	for _, record := range recent {
		summary.RecentUsageCount += 1
		if record.Status == "succeeded" {
			summary.RecentSuccessCount += 1
		} else {
			summary.RecentFailedCount += 1
		}
		summary.RecentImageCount += int64(record.Count)
		summary.RecentCreditsUsed += record.CreditsUsed
		user := userByID[record.UserID]
		recentWithUser = append(recentWithUser, businessUsageRecordWithUser{
			UsageRecord: record,
			UID:         user.UID,
			Username:    user.Username,
			Email:       user.Email,
		})
	}

	writeJSON(w, http.StatusOK, businessDashboardResponse{
		Summary:    summary,
		Recent:     recentWithUser,
		TopUsers:   topUsers,
		ModelUsage: modelUsage,
		Page:       paginationMeta{Page: 1, PageSize: 10, Total: recentTotal},
	})
}

func (s *Server) handleListBusinessUsers(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()

	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	users, err := store.ListUsers(r.Context(), businessauth.ListUsersOptions{
		IncludeDeleted: boolQueryValue(r, "includeDeleted"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	usages, err := imageStore.UserUsage(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	credits, err := creditStore.Summaries(r.Context(), userIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	items := make([]businessUserWithUsage, 0, len(users))
	for _, user := range users {
		usage := usages[user.ID]
		usage.UserID = user.ID
		credit := credits[user.ID]
		credit.UserID = user.ID
		items = append(items, businessUserWithUsage{
			User:   user,
			Usage:  usage,
			Credit: credit,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) purgeExpiredDeletedBusinessUsers(r *http.Request, userStore *businessauth.Store) error {
	cutoff := time.Now().UTC().Add(-businessauth.DeletedUserRetention)
	users, err := userStore.DeletedUsersBefore(r.Context(), cutoff)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return nil
	}

	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		return err
	}
	defer imageStore.Close()
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		return err
	}
	defer creditStore.Close()

	for _, user := range users {
		if _, ok, err := s.purgeDeletedBusinessUser(r, userStore, imageStore, creditStore, user.ID); err != nil {
			return err
		} else if !ok {
			return errors.New("deleted user purge failed")
		}
	}
	return nil
}

func (s *Server) purgeDeletedBusinessUser(
	r *http.Request,
	userStore *businessauth.Store,
	imageStore *businessimage.Store,
	creditStore *businesscredits.Store,
	userID string,
) (businessauth.User, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return businessauth.User{}, false, nil
	}
	user, ok, err := userStore.GetUserByID(r.Context(), userID)
	if err != nil || !ok {
		return user, ok, err
	}
	if user.Status != businessauth.StatusDeleted {
		return user, true, errBusinessUserNotDeleted
	}
	if err := s.clearBusinessUserData(r, imageStore, creditStore, user.ID); err != nil {
		return businessauth.User{}, false, err
	}
	return userStore.PurgeDeletedUser(r.Context(), user.ID)
}

func (s *Server) clearBusinessUserData(
	r *http.Request,
	imageStore *businessimage.Store,
	creditStore *businesscredits.Store,
	userID string,
) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	fileNames, err := imageStore.ImageFileNamesForUser(r.Context(), userID)
	if err != nil {
		return err
	}
	if err := imageStore.DeleteAssetsByUser(r.Context(), userID); err != nil {
		return err
	}
	if _, err := imageStore.ClearConversations(r.Context(), userID); err != nil {
		return err
	}
	if _, err := s.deleteUnreferencedBusinessImageFiles(r, imageStore, fileNames); err != nil {
		return err
	}
	if err := creditStore.DeleteUserCreditData(r.Context(), userID); err != nil {
		return err
	}
	trackerStore, err := s.newBusinessTrackerStore()
	if err != nil {
		return err
	}
	defer trackerStore.Close()
	return trackerStore.DeleteUserRecords(r.Context(), userID)
}

func (s *Server) handleGetBusinessUserDetail(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}

	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, userStore); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	user, ok, err := userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}

	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	usages, err := imageStore.UserUsage(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	usage := usages[user.ID]
	usage.UserID = user.ID

	usagePage, usagePageSize, usageOffset := paginationFromQuery(r, "usage", 20, 100)
	assetPage, assetPageSize, assetOffset := paginationFromQuery(r, "assets", 20, 100)
	ledgerPage, ledgerPageSize, ledgerOffset := paginationFromQuery(r, "ledger", 20, 100)

	recentUsage, recentUsageTotal, err := imageStore.UsageRecordsPage(r.Context(), usageRecordFilterFromQuery(r, user.ID), usagePageSize, usageOffset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	recentUsageWithUser := make([]businessUsageRecordWithUser, 0, len(recentUsage))
	for _, record := range recentUsage {
		recentUsageWithUser = append(recentUsageWithUser, businessUsageRecordWithUser{
			UsageRecord: record,
			UID:         user.UID,
			Username:    user.Username,
			Email:       user.Email,
		})
	}
	recentAssets, recentAssetsTotal, err := imageStore.AssetsByUserPage(r.Context(), user.ID, assetPageSize, assetOffset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	conversationCount, err := imageStore.ConversationCount(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	credit, err := creditStore.Summary(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	ledger, ledgerTotal, err := creditStore.LedgerEntriesPage(r.Context(), user.ID, ledgerPageSize, ledgerOffset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, businessUserDetailResponse{
		User:               user,
		Usage:              usage,
		Credit:             credit,
		RecentUsage:        recentUsageWithUser,
		RecentUsagePage:    paginationMeta{Page: usagePage, PageSize: usagePageSize, Total: recentUsageTotal},
		RecentAssets:       recentAssets,
		RecentAssetsPage:   paginationMeta{Page: assetPage, PageSize: assetPageSize, Total: recentAssetsTotal},
		RecentCreditLedger: ledger,
		RecentLedgerPage:   paginationMeta{Page: ledgerPage, PageSize: ledgerPageSize, Total: ledgerTotal},
		ConversationCount:  conversationCount,
	})
}

func (s *Server) handleGetBusinessMe(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	user, ok, err := userStore.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	credit, err := creditStore.Summary(r.Context(), session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, businessMeResponse{User: user, Credit: credit})
}

func (s *Server) handleChangeBusinessMePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if len(strings.TrimSpace(body.NewPassword)) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "new password must be at least 6 characters"})
		return
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	user, ok, err := store.ChangeUserPassword(r.Context(), session.UserID, body.CurrentPassword, body.NewPassword)
	if errors.Is(err, businessauth.ErrInvalidCurrentPassword) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "current password is invalid"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleListBusinessUsage(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	items, total, err := imageStore.UsageRecordsPage(r.Context(), usageRecordFilterFromQuery(r, session.UserID), pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func (s *Server) handleListAllBusinessUsage(w http.ResponseWriter, r *http.Request) {
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()

	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	users, err := userStore.ListUsers(r.Context(), businessauth.ListUsersOptions{IncludeDeleted: true})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	userByID := make(map[string]businessauth.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}

	filter := usageRecordFilterFromQuery(r, "")
	userQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("userQuery")))
	if filter.UserID == "" && userQuery != "" {
		matchedUserIDs := make([]string, 0)
		for _, user := range users {
			if businessUserMatchesQuery(user, userQuery) {
				matchedUserIDs = append(matchedUserIDs, user.ID)
			}
		}
		if len(matchedUserIDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []businessUsageRecordWithUser{},
				"page":  paginationMeta{Page: 1, PageSize: intQueryValue(r, "pageSize", 20), Total: 0},
			})
			return
		}
		filter.UserIDs = matchedUserIDs
	}
	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	records, total, err := imageStore.UsageRecordsPage(r.Context(), filter, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	items := make([]businessUsageRecordWithUser, 0, len(records))
	for _, record := range records {
		user := userByID[record.UserID]
		items = append(items, businessUsageRecordWithUser{
			UsageRecord: record,
			UID:         user.UID,
			Username:    user.Username,
			Email:       user.Email,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func businessUserMatchesQuery(user businessauth.User, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	values := []string{
		user.ID,
		strconv.FormatInt(user.UID, 10),
		user.Email,
		user.Username,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (s *Server) handleCreateBusinessUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Balance  *int64 `json:"balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	email := firstNonEmpty(body.Email, body.Username)
	role := strings.TrimSpace(body.Role)
	if role == "" {
		role = businessauth.RoleUser
	}
	user, err := store.CreateUserWithUsername(r.Context(), email, body.Username, body.Password, role)
	if errors.Is(err, businessauth.ErrUserAlreadyExists) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "email or username already exists"})
		return
	}
	if errors.Is(err, businessauth.ErrUserDeleted) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "user is deleted and can be restored within 7 days"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	settings := s.businessSystemSettings(r)
	initialBalance := settings.User.DefaultCredits
	if body.Balance != nil {
		initialBalance = *body.Balance
	}
	if initialBalance < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "balance must be non-negative"})
		return
	}
	if initialBalance > 0 {
		creditStore, err := s.newBusinessCreditStore()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
			return
		}
		defer creditStore.Close()
		if _, _, err := creditStore.SetBalance(r.Context(), user.ID, initialBalance, "new_user_default"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": user})
}

func (s *Server) handleUpdateBusinessUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	user, ok, err := store.UpdateUser(r.Context(), r.PathValue("id"), body.Username, body.Password)
	if errors.Is(err, businessauth.ErrUserAlreadyExists) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "username already exists"})
		return
	}
	if errors.Is(err, businessauth.ErrUserDeleted) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "deleted user cannot be edited"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": user})
}

func (s *Server) handleUpdateBusinessUserStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	userID := strings.TrimSpace(r.PathValue("id"))
	status := strings.ToLower(strings.TrimSpace(body.Status))
	if status != businessauth.StatusActive && status != businessauth.StatusDisabled {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "status must be active or disabled"})
		return
	}
	if session, ok := requestAuthSession(r); ok && session.UserID == userID && status == businessauth.StatusDisabled {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot disable current user"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	user, ok, err := store.SetUserStatus(r.Context(), userID, status)
	if errors.Is(err, businessauth.ErrUserDeleted) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "deleted user cannot be enabled or disabled"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": user})
}

func (s *Server) handleDeleteBusinessUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}
	if session, ok := requestAuthSession(r); ok && session.UserID == userID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot delete current user"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	user, ok, err := store.DeleteUser(r.Context(), userID)
	if errors.Is(err, businessauth.ErrLastAdminUser) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot delete the last admin user"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": user})
}

func (s *Server) handleRestoreBusinessUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	user, ok, err := store.RestoreUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": user})
}

func (s *Server) handlePurgeBusinessUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}
	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()

	user, ok, err := s.purgeDeletedBusinessUser(r, store, imageStore, creditStore, userID)
	if errors.Is(err, errBusinessUserNotDeleted) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user must be deleted before permanent deletion"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": user})
}

func (s *Server) handleClearBusinessUserData(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}
	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, userStore); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	user, ok, err := userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	imageStore, err := s.newBusinessImageStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "usage store failed"})
		return
	}
	defer imageStore.Close()
	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	if err := s.clearBusinessUserData(r, imageStore, creditStore, user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "item": user})
}

func (s *Server) handleGetBusinessCredit(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	store, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer store.Close()
	summary, err := store.Summary(r.Context(), session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleListBusinessCreditLedger(w http.ResponseWriter, r *http.Request) {
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	page, pageSize, offset := paginationFromQuery(r, "", 10, 50)
	store, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer store.Close()
	items, total, err := store.BalanceLedgerEntriesPage(r.Context(), session.UserID, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, businessCreditLedgerResponse{
		Items: items,
		Page:  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func (s *Server) handleSetBusinessUserCredit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Balance   *int64 `json:"balance"`
		Amount    int64  `json:"amount"`
		Operation string `json:"operation"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	userID := strings.TrimSpace(r.PathValue("id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "user id is required"})
		return
	}
	if body.Balance != nil && *body.Balance < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "balance must be non-negative"})
		return
	}

	userStore, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer userStore.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, userStore); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if user, ok, err := userStore.GetUserByID(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	} else if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	} else if user.Status == businessauth.StatusDeleted {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "deleted user credit cannot be adjusted"})
		return
	}

	creditStore, err := s.newBusinessCreditStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "credit store failed"})
		return
	}
	defer creditStore.Close()
	var (
		summary businesscredits.Summary
		entry   businesscredits.LedgerEntry
	)
	operation := strings.ToLower(strings.TrimSpace(body.Operation))
	switch operation {
	case "recharge":
		if body.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "amount must be positive"})
			return
		}
		summary, entry, err = creditStore.Add(r.Context(), userID, body.Amount, businesscredits.ReasonAdminRecharge, "", false)
	case "refund":
		if body.Amount <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "amount must be positive"})
			return
		}
		summary, entry, err = creditStore.Add(r.Context(), userID, -body.Amount, businesscredits.ReasonAdminRefund, "", true)
	default:
		if body.Balance == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "operation must be recharge or refund"})
			return
		}
		summary, entry, err = creditStore.SetBalance(r.Context(), userID, *body.Balance, body.Reason)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credit": summary, "ledger": entry})
}

func (s *Server) handleResetBusinessUserPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	store, err := s.newBusinessAuthStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "user store failed"})
		return
	}
	defer store.Close()
	if err := s.purgeExpiredDeletedBusinessUsers(r, store); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	user, ok, err := store.ResetUserPassword(r.Context(), r.PathValue("id"), body.Password)
	if errors.Is(err, businessauth.ErrUserDeleted) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "deleted user password cannot be reset"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": user})
}
