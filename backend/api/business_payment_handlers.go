package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"imagestudio/internal/businesspayments"
)

type paymentPackagePayload struct {
	PackageType  string `json:"packageType"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AmountCents  int64  `json:"amountCents"`
	Credits      int64  `json:"credits"`
	DurationDays int    `json:"durationDays"`
	Currency     string `json:"currency"`
	Enabled      bool   `json:"enabled"`
	SortOrder    int    `json:"sortOrder"`
}

type paymentProviderPayload struct {
	ProviderKey      string            `json:"providerKey"`
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	SupportedMethods []string          `json:"supportedMethods"`
	Config           map[string]string `json:"config"`
	SortOrder        int               `json:"sortOrder"`
}

func (s *Server) handleListPaymentPackages(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListPackages(r.Context(), false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetBusinessSubscription(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, err := store.GetCurrentSubscription(r.Context(), businessUserIDForRequest(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}

func (s *Server) handleAdminGetBusinessSubscription(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, err := store.GetCurrentSubscription(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscription": item})
}

func (s *Server) handleAdminListBusinessSubscriptions(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	items, total, err := store.ListSubscriptions(r.Context(), businesspayments.SubscriptionFilters{
		Status:       r.URL.Query().Get("status"),
		Search:       r.URL.Query().Get("search"),
		ActiveWindow: r.URL.Query().Get("activeWindow"),
	}, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func (s *Server) handleAdminListPaymentPackages(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListPackages(r.Context(), true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminListPaymentProviders(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListProviders(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	items, err := store.AvailablePaymentMethods(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminCreatePaymentProvider(w http.ResponseWriter, r *http.Request) {
	var body paymentProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, err := store.CreateProvider(r.Context(), paymentProviderInput(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleAdminUpdatePaymentProvider(w http.ResponseWriter, r *http.Request) {
	var body paymentProviderPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.UpdateProvider(r.Context(), r.PathValue("id"), paymentProviderInput(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminDeletePaymentProvider(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	ok, err := store.DeleteProvider(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAdminCreatePaymentPackage(w http.ResponseWriter, r *http.Request) {
	var body paymentPackagePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, err := store.CreatePackage(r.Context(), paymentPackageInput(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) handleAdminUpdatePaymentPackage(w http.ResponseWriter, r *http.Request) {
	var body paymentPackagePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.UpdatePackage(r.Context(), r.PathValue("id"), paymentPackageInput(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "package not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleAdminDeletePaymentPackage(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	ok, err := store.DeletePackage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "package not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreatePaymentOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PackageID     string `json:"packageId"`
		PaymentMethod string `json:"paymentMethod"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	session, ok := requestAuthSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	order, err := store.CreateOrder(r.Context(), businesspayments.CreateOrderInput{
		UserID:        session.UserID,
		Username:      session.Username,
		UserEmail:     session.Email,
		PackageID:     body.PackageID,
		PaymentMethod: body.PaymentMethod,
		NotifyBaseURL: publicRequestBaseURL(r),
		ReturnURL:     publicRequestBaseURL(r) + "/credits",
		ClientIP:      clientIPFromRequest(r),
		UserAgent:     r.UserAgent(),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order})
}

func (s *Server) handleEasyPayWebhook(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.RawQuery
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
			return
		}
		if strings.TrimSpace(string(body)) != "" {
			raw = string(body)
		}
	}
	if strings.TrimSpace(raw) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty callback"})
		return
	}
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	if _, err := store.HandleEasyPayNotification(r.Context(), raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	_, _ = w.Write([]byte("success"))
}

func (s *Server) handleListPaymentOrders(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	items, total, err := store.ListOrders(r.Context(), businesspayments.OrderFilters{
		UserID: businessUserIDForRequest(r),
		Status: r.URL.Query().Get("status"),
		Kind:   r.URL.Query().Get("kind"),
		Search: r.URL.Query().Get("search"),
	}, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func (s *Server) handleAdminListPaymentOrders(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	page, pageSize, offset := paginationFromQuery(r, "", 20, 100)
	items, total, err := store.ListOrders(r.Context(), businesspayments.OrderFilters{
		Status: r.URL.Query().Get("status"),
		Kind:   r.URL.Query().Get("kind"),
		Search: r.URL.Query().Get("search"),
	}, pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"page":  paginationMeta{Page: page, PageSize: pageSize, Total: total},
	})
}

func (s *Server) handleAdminCompletePaymentOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProviderTradeNo   string `json:"providerTradeNo"`
		CommissionRateBPS int    `json:"commissionRateBps"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	session, _ := requestAuthSession(r)
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	result, err := store.CompleteOrder(r.Context(), businesspayments.CompleteOrderInput{
		OrderID:           r.PathValue("id"),
		Operator:          session.UserID,
		ProviderTradeNo:   body.ProviderTradeNo,
		CommissionRateBPS: body.CommissionRateBPS,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminCancelPaymentOrder(w http.ResponseWriter, r *http.Request) {
	session, _ := requestAuthSession(r)
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	item, ok, err := store.CancelOrder(r.Context(), r.PathValue("id"), session.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "order not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": item})
}

func (s *Server) handleAdminRefundPaymentOrder(w http.ResponseWriter, r *http.Request) {
	session, _ := requestAuthSession(r)
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	result, err := store.RefundOrder(r.Context(), r.PathValue("id"), session.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": paymentErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminListPaymentOrderAuditLogs(w http.ResponseWriter, r *http.Request) {
	store, err := s.newBusinessPaymentStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "payment store failed"})
		return
	}
	defer store.Close()
	items, err := store.ListAuditLogs(r.Context(), r.PathValue("id"), intQuery(r, "limit", 100))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func paymentPackageInput(body paymentPackagePayload) businesspayments.PackageInput {
	return businesspayments.PackageInput{
		PackageType:  body.PackageType,
		Name:         body.Name,
		Description:  body.Description,
		AmountCents:  body.AmountCents,
		Credits:      body.Credits,
		DurationDays: body.DurationDays,
		Currency:     body.Currency,
		Enabled:      body.Enabled,
		SortOrder:    body.SortOrder,
	}
}

func paymentProviderInput(body paymentProviderPayload) businesspayments.ProviderInput {
	return businesspayments.ProviderInput{
		ProviderKey:      body.ProviderKey,
		Name:             body.Name,
		Enabled:          body.Enabled,
		SupportedMethods: body.SupportedMethods,
		Config:           body.Config,
		SortOrder:        body.SortOrder,
	}
}

func publicRequestBaseURL(r *http.Request) string {
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if strings.TrimSpace(host) == "" {
		return ""
	}
	return strings.TrimRight(proto+"://"+host, "/")
}

func clientIPFromRequest(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if value != "" {
			return value
		}
	}
	host := r.RemoteAddr
	if parsed, err := url.Parse("//" + r.RemoteAddr); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	return strings.TrimSpace(host)
}

func paymentErrorMessage(err error) string {
	switch {
	case errors.Is(err, businesspayments.ErrPackageInvalid):
		return "充值套餐无效"
	case errors.Is(err, businesspayments.ErrPackageDisabled):
		return "充值套餐已停用"
	case errors.Is(err, businesspayments.ErrPackageInUse):
		return "套餐已有订单记录，不能删除；可以改为停用"
	case errors.Is(err, businesspayments.ErrOrderInvalid):
		return "订单无效"
	case errors.Is(err, businesspayments.ErrOrderNotPayable):
		return "订单当前状态不能完成支付"
	case errors.Is(err, businesspayments.ErrOrderNotRefundable):
		return "订单当前状态不能退款"
	case errors.Is(err, businesspayments.ErrSubscriptionChange):
		return "当前订阅周期内不能降级套餐"
	case errors.Is(err, businesspayments.ErrSubscriptionDurationMismatch):
		return "只能升级相同订阅周期的套餐"
	case errors.Is(err, businesspayments.ErrProviderInvalid):
		return "支付渠道配置无效"
	case errors.Is(err, businesspayments.ErrProviderUnavailable):
		return "当前支付方式不可用"
	case errors.Is(err, businesspayments.ErrProviderInUse):
		return "渠道存在待处理订单，不能停用或删除"
	default:
		message := strings.TrimSpace(err.Error())
		if message == "" {
			return "支付操作失败"
		}
		return message
	}
}
