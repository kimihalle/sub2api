package handler

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	sanbaoDefaultBaseURL        = "https://sanbaobeauty.com"
	sanbaoCreateVideoPath       = "/openapi/v1/videos"
	sanbaoVideoStatusQueued     = "queued"
	sanbaoVideoStatusProcessing = "processing"
	sanbaoVideoStatusSucceeded  = "succeeded"
	sanbaoVideoStatusFailed     = "failed"
)

type sanbaoVideoTask struct {
	TaskID          string
	UserID          int64
	APIKeyID        int64
	AccountID       int64
	GroupID         *int64
	Model           string
	UpstreamModel   string
	Status          string
	Ratio           string
	Resolution      string
	DurationSeconds int
	VideoURL        string
	DownloadURL     string
	ErrorMessage    string
	Cost            float64
	RefundAmount    *float64
	RefundedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type sanbaoVideoListItem struct {
	ID              int64      `json:"id"`
	TaskID          string     `json:"task_id"`
	UserID          int64      `json:"user_id"`
	APIKeyID        int64      `json:"api_key_id"`
	APIKeyName      string     `json:"api_key_name,omitempty"`
	AccountID       int64      `json:"account_id"`
	AccountName     string     `json:"account_name,omitempty"`
	GroupID         *int64     `json:"group_id,omitempty"`
	GroupName       string     `json:"group_name,omitempty"`
	Model           string     `json:"model"`
	UpstreamModel   string     `json:"upstream_model,omitempty"`
	Prompt          string     `json:"prompt,omitempty"`
	Status          string     `json:"status"`
	Ratio           string     `json:"ratio,omitempty"`
	Resolution      string     `json:"resolution,omitempty"`
	DurationSeconds int        `json:"duration_seconds,omitempty"`
	VideoURL        string     `json:"video_url,omitempty"`
	DownloadURL     string     `json:"download_url,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	Cost            float64    `json:"cost"`
	RefundAmount    *float64   `json:"refund_amount,omitempty"`
	RefundedAt      *time.Time `json:"refunded_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func newSanbaoVideoDB(cfg *config.Config) *sql.DB {
	if cfg == nil || strings.TrimSpace(cfg.Database.Host) == "" {
		return nil
	}
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(6)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)
	return db
}

func (h *OpenAIGatewayHandler) SanbaoVideoGeneration(c *gin.Context) {
	start := time.Now()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if h.sanbaoDB == nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Database is not configured")
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body must be valid JSON")
		return
	}
	requestModel := strings.TrimSpace(jsonString(payload["model"]))
	if requestModel == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	if !service.IsSanbaoVideoModel(requestModel) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this model")
		return
	}
	prompt := strings.TrimSpace(jsonString(payload["prompt"]))
	if prompt == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "prompt is required")
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	account, err := h.selectSanbaoVideoAccount(c.Request.Context(), apiKey.GroupID)
	if err != nil || account == nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "no_available_account", "No available Sanbao video account")
		return
	}

	upstreamModel := account.GetMappedModel(requestModel)
	if upstreamModel == "" {
		upstreamModel = requestModel
	}
	upstreamPayload := normalizeSanbaoVideoPayload(payload, upstreamModel)
	upstreamBody, _ := json.Marshal(upstreamPayload)
	respBytes, statusCode, headers, err := h.doSanbaoJSON(c.Request.Context(), account, http.MethodPost, sanbaoCreateVideoPath, upstreamBody)
	if err != nil {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, upstreamModel, false, nil)
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, upstreamModel, false, nil)
		writeSanbaoUpstreamError(c, statusCode, headers, respBytes)
		return
	}
	h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, upstreamModel, true, nil)

	taskID := firstGJSON(respBytes, "data.id", "id", "task_id", "data.task_id")
	if taskID == "" {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream response missing task id")
		return
	}
	status := normalizeSanbaoTaskStatus(firstGJSON(respBytes, "data.status", "status"))
	ratio := strings.TrimSpace(jsonString(upstreamPayload["ratio"]))
	resolution := service.NormalizeVideoBillingResolutionOrDefault(jsonString(upstreamPayload["resolution"]))
	durationSeconds := jsonInt(upstreamPayload["duration"], 5)

	result := &service.OpenAIForwardResult{
		RequestID:            taskID,
		ResponseID:           taskID,
		Model:                requestModel,
		BillingModel:         upstreamModel,
		UpstreamModel:        upstreamModel,
		UpstreamEndpoint:     sanbaoCreateVideoPath,
		Duration:             time.Since(start),
		VideoCount:           1,
		VideoResolution:      resolution,
		VideoDurationSeconds: durationSeconds,
	}
	user := apiKey.User
	if user == nil {
		user = &service.User{ID: subject.UserID}
	}
	if err := h.gatewayService.RecordUsage(c.Request.Context(), &service.OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               user,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    "/v1/videos/generations",
		UpstreamEndpoint:   sanbaoCreateVideoPath,
		UserAgent:          c.GetHeader("User-Agent"),
		IPAddress:          ip.GetClientIP(c),
		RequestPayloadHash: service.HashUsageRequestPayload(body),
		APIKeyService:      h.apiKeyService,
		QuotaPlatform:      service.QuotaPlatform(c.Request.Context(), apiKey),
		ChannelUsageFields: service.ChannelUsageFields{
			OriginalModel:      requestModel,
			ChannelMappedModel: upstreamModel,
		},
	}); err != nil {
		h.errorResponse(c, http.StatusPaymentRequired, "billing_error", err.Error())
		return
	}
	usageRequestID := sanbaoUsageBillingRequestID(c.Request.Context(), taskID)
	cost := h.lookupSanbaoUsageCost(c.Request.Context(), apiKey.ID, usageRequestID)
	if cost <= 0 && usageRequestID != taskID {
		cost = h.lookupSanbaoUsageCost(c.Request.Context(), apiKey.ID, taskID)
	}
	if err := h.upsertSanbaoVideoTask(c.Request.Context(), &sanbaoVideoTask{
		TaskID:          taskID,
		UserID:          subject.UserID,
		APIKeyID:        apiKey.ID,
		AccountID:       account.ID,
		GroupID:         apiKey.GroupID,
		Model:           requestModel,
		UpstreamModel:   upstreamModel,
		Status:          status,
		Ratio:           ratio,
		Resolution:      resolution,
		DurationSeconds: durationSeconds,
		VideoURL:        sanbaoVideoURLFromResponse(respBytes),
		DownloadURL:     sanbaoDownloadURLFromResponse(respBytes),
		ErrorMessage:    firstGJSON(respBytes, "data.error", "data.message", "error", "message"),
		Cost:            cost,
	}, upstreamPayload, respBytes); err != nil {
		requestLogger(c, "handler.openai_gateway.sanbao_video").Warn("sanbao_video.save_task_failed", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       taskID,
		"task_id":  taskID,
		"status":   normalizeSanbaoDownstreamStatus(status),
		"object":   "video.generation",
		"poll_url": "/v1/videos/generations/" + taskID,
		"data":     jsonRawObject(respBytes),
	})
}

func (h *OpenAIGatewayHandler) SanbaoVideoStatus(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("request_id"))
	if taskID == "" {
		taskID = strings.TrimSpace(c.Param("task_id"))
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	if h.sanbaoDB == nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	task, err := h.getSanbaoVideoTask(c.Request.Context(), taskID, subject.UserID, apiKey.ID)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return
	}
	account, err := h.gatewayService.GetAccountByID(c.Request.Context(), task.AccountID)
	if err != nil || account == nil || !isSanbaoAccount(account) {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task account not found")
		return
	}
	respBytes, statusCode, headers, err := h.doSanbaoJSON(c.Request.Context(), account, http.MethodGet, sanbaoCreateVideoPath+"/"+taskID, nil)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		writeSanbaoUpstreamError(c, statusCode, headers, respBytes)
		return
	}
	updated := updateTaskFromSanbaoResponse(task, respBytes)
	_ = h.updateSanbaoVideoTaskFromStatus(c.Request.Context(), updated, respBytes)
	if strings.EqualFold(updated.Status, sanbaoVideoStatusFailed) {
		_ = h.refundSanbaoVideoTask(c.Request.Context(), updated)
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           taskID,
		"task_id":      taskID,
		"status":       normalizeSanbaoDownstreamStatus(updated.Status),
		"progress":     gjson.GetBytes(respBytes, "data.progress").Value(),
		"video_url":    updated.VideoURL,
		"download_url": updated.DownloadURL,
		"error":        updated.ErrorMessage,
		"data":         jsonRawObject(respBytes),
	})
}

func (h *OpenAIGatewayHandler) SanbaoVideoContent(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("request_id"))
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	if h.sanbaoDB == nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
		return
	}
	task, err := h.getSanbaoVideoTask(c.Request.Context(), taskID, subject.UserID, apiKey.ID)
	if err != nil || (strings.TrimSpace(task.DownloadURL) == "" && strings.TrimSpace(task.VideoURL) == "") {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video content not found")
		return
	}
	contentURL := strings.TrimSpace(task.DownloadURL)
	if contentURL == "" {
		contentURL = strings.TrimSpace(task.VideoURL)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, contentURL, nil)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Invalid video URL")
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer resp.Body.Close()
	for k, values := range resp.Header {
		for _, v := range values {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func (h *OpenAIGatewayHandler) SanbaoVideoLogs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	if h.sanbaoDB == nil {
		c.JSON(http.StatusOK, gin.H{"data": []sanbaoVideoListItem{}, "pagination": gin.H{"page": 1, "page_size": 20, "total": 0}})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	status := strings.TrimSpace(c.Query("status"))
	model := strings.TrimSpace(c.Query("model"))
	args := []any{subject.UserID}
	where := "WHERE t.user_id = $1"
	if status != "" && status != "all" {
		args = append(args, status)
		where += fmt.Sprintf(" AND t.status = $%d", len(args))
	}
	if model != "" {
		args = append(args, "%"+model+"%")
		where += fmt.Sprintf(" AND (t.model ILIKE $%d OR t.upstream_model ILIKE $%d)", len(args), len(args))
	}
	var total int
	_ = h.sanbaoDB.QueryRowContext(c.Request.Context(), "SELECT COUNT(*) FROM sanbao_video_tasks t "+where, args...).Scan(&total)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := h.sanbaoDB.QueryContext(c.Request.Context(), `
		SELECT t.id, t.task_id, t.user_id, t.api_key_id, COALESCE(k.name, ''), t.account_id, COALESCE(a.name, ''),
		       t.group_id, COALESCE(g.name, ''), t.model, COALESCE(t.upstream_model, ''), COALESCE(t.prompt, ''),
		       t.status, COALESCE(t.ratio, ''), COALESCE(t.resolution, ''), t.duration_seconds,
		       COALESCE(t.video_url, ''), COALESCE(t.download_url, ''), COALESCE(t.error_message, ''),
		       t.cost, t.refund_amount, t.refunded_at, t.created_at, t.updated_at
		FROM sanbao_video_tasks t
		LEFT JOIN api_keys k ON k.id = t.api_key_id
		LEFT JOIN accounts a ON a.id = t.account_id
		LEFT JOIN groups g ON g.id = t.group_id
		`+where+`
		ORDER BY t.created_at DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	items := make([]sanbaoVideoListItem, 0)
	for rows.Next() {
		var item sanbaoVideoListItem
		var groupName string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.UserID, &item.APIKeyID, &item.APIKeyName, &item.AccountID, &item.AccountName,
			&item.GroupID, &groupName, &item.Model, &item.UpstreamModel, &item.Prompt, &item.Status, &item.Ratio, &item.Resolution,
			&item.DurationSeconds, &item.VideoURL, &item.DownloadURL, &item.ErrorMessage, &item.Cost, &item.RefundAmount,
			&item.RefundedAt, &item.CreatedAt, &item.UpdatedAt); err == nil {
			if item.GroupID != nil {
				item.GroupName = groupName
			}
			items = append(items, item)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "pagination": gin.H{"page": page, "page_size": pageSize, "total": total}})
}

var sanbaoPollerStartOnce sync.Once

func (h *OpenAIGatewayHandler) startSanbaoVideoPoller() {
	if h == nil || h.sanbaoDB == nil || h.gatewayService == nil {
		return
	}
	sanbaoPollerStartOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(45 * time.Second)
			defer ticker.Stop()
			// First pass is delayed a little so migrations can finish during startup.
			time.Sleep(10 * time.Second)
			for {
				h.pollPendingSanbaoVideoTasks(context.Background(), 20)
				<-ticker.C
			}
		}()
	})
}

func (h *OpenAIGatewayHandler) pollPendingSanbaoVideoTasks(parent context.Context, limit int) {
	if h == nil || h.sanbaoDB == nil || h.gatewayService == nil || limit <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	tasks, err := h.listPendingSanbaoVideoTasks(ctx, limit)
	if err != nil {
		return
	}
	for _, task := range tasks {
		account, err := h.gatewayService.GetAccountByID(ctx, task.AccountID)
		if err != nil || account == nil || !isSanbaoAccount(account) {
			continue
		}
		respBytes, statusCode, _, err := h.doSanbaoJSON(ctx, account, http.MethodGet, sanbaoCreateVideoPath+"/"+task.TaskID, nil)
		if err != nil || statusCode < 200 || statusCode >= 300 {
			continue
		}
		updated := updateTaskFromSanbaoResponse(task, respBytes)
		_ = h.updateSanbaoVideoTaskFromStatus(ctx, updated, respBytes)
		if strings.EqualFold(updated.Status, sanbaoVideoStatusFailed) {
			_ = h.refundSanbaoVideoTask(ctx, updated)
		}
	}
}

func (h *OpenAIGatewayHandler) listPendingSanbaoVideoTasks(ctx context.Context, limit int) ([]*sanbaoVideoTask, error) {
	if h.sanbaoDB == nil {
		return nil, nil
	}
	rows, err := h.sanbaoDB.QueryContext(ctx, `
		SELECT task_id, user_id, api_key_id, account_id, group_id, model, COALESCE(upstream_model,''), status,
		       COALESCE(ratio,''), COALESCE(resolution,''), duration_seconds, COALESCE(video_url,''), COALESCE(download_url,''),
		       COALESCE(error_message,''), cost, refund_amount, refunded_at, created_at, updated_at
		FROM sanbao_video_tasks
		WHERE status IN ('queued','processing','in_progress')
		  AND updated_at < NOW() - INTERVAL '15 seconds'
		ORDER BY updated_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*sanbaoVideoTask, 0)
	for rows.Next() {
		task := &sanbaoVideoTask{}
		if err := rows.Scan(&task.TaskID, &task.UserID, &task.APIKeyID, &task.AccountID, &task.GroupID, &task.Model, &task.UpstreamModel,
			&task.Status, &task.Ratio, &task.Resolution, &task.DurationSeconds, &task.VideoURL, &task.DownloadURL, &task.ErrorMessage,
			&task.Cost, &task.RefundAmount, &task.RefundedAt, &task.CreatedAt, &task.UpdatedAt); err == nil {
			out = append(out, task)
		}
	}
	return out, rows.Err()
}

func (h *OpenAIGatewayHandler) selectSanbaoVideoAccount(ctx context.Context, groupID *int64) (*service.Account, error) {
	if h == nil || h.sanbaoDB == nil || h.gatewayService == nil || groupID == nil {
		return nil, sql.ErrNoRows
	}
	var accountID int64
	err := h.sanbaoDB.QueryRowContext(ctx, `
		SELECT a.id
		FROM accounts a
		JOIN account_groups ag ON ag.account_id = a.id
		WHERE ag.group_id = $1
		  AND a.deleted_at IS NULL
		  AND a.platform = 'openai'
		  AND a.type = 'apikey'
		  AND a.status = 'active'
		  AND COALESCE(a.schedulable, true) = true
		  AND (
		    LOWER(COALESCE(a.credentials->>'base_url', '')) LIKE '%sanbaobeauty.com%'
		    OR LOWER(COALESCE(a.credentials->>'provider', '')) IN ('sanbao', '三宝')
		  )
		ORDER BY ag.priority ASC, a.priority DESC, a.id ASC
		LIMIT 1
	`, *groupID).Scan(&accountID)
	if err != nil {
		return nil, err
	}
	return h.gatewayService.GetAccountByID(ctx, accountID)
}

func isSanbaoAccount(account *service.Account) bool {
	if account == nil || account.Type != service.AccountTypeAPIKey {
		return false
	}
	base := strings.ToLower(account.GetCredential("base_url"))
	if strings.Contains(base, "sanbaobeauty.com") {
		return true
	}
	provider := strings.ToLower(account.GetCredential("provider"))
	return provider == "sanbao" || provider == "涓夊疂"
}

func normalizeSanbaoVideoPayload(payload map[string]any, upstreamModel string) map[string]any {
	out := make(map[string]any, len(payload)+2)
	for k, v := range payload {
		out[k] = v
	}
	out["model"] = upstreamModel
	if out["ratio"] == nil {
		if v := firstAny(out, "aspect_ratio", "aspectRatio", "size"); v != nil && strings.Contains(jsonString(v), ":") {
			out["ratio"] = jsonString(v)
		}
	}
	if out["duration"] == nil {
		if v := firstAny(out, "duration_seconds", "durationSeconds"); v != nil {
			out["duration"] = v
		}
	}
	if out["images"] == nil {
		if v := firstAny(out, "image", "image_url", "imageUrl", "image_urls", "imageUrls", "reference_images", "referenceImages"); v != nil {
			out["images"] = normalizeSanbaoImages(v)
		}
	}
	delete(out, "async")
	delete(out, "stream")
	delete(out, "response_format")
	delete(out, "aspect_ratio")
	delete(out, "aspectRatio")
	delete(out, "duration_seconds")
	delete(out, "durationSeconds")
	delete(out, "image")
	delete(out, "image_url")
	delete(out, "imageUrl")
	delete(out, "image_urls")
	delete(out, "imageUrls")
	delete(out, "reference_images")
	delete(out, "referenceImages")
	if _, ok := out["resolution"]; !ok {
		out["resolution"] = "720p"
	}
	return out
}

func normalizeSanbaoImages(v any) any {
	switch t := v.(type) {
	case []any:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return []any{}
		}
		return []any{t}
	default:
		return v
	}
}

func (h *OpenAIGatewayHandler) doSanbaoJSON(ctx context.Context, account *service.Account, method, path string, body []byte) ([]byte, int, http.Header, error) {
	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if apiKey == "" {
		apiKey = strings.TrimSpace(account.GetCredential("api_key"))
	}
	if apiKey == "" {
		return nil, 0, nil, errors.New("Sanbao upstream API key is empty")
	}
	base := sanbaoRootBaseURL(account)
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return respBody, resp.StatusCode, resp.Header.Clone(), nil
}

func sanbaoRootBaseURL(account *service.Account) string {
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if base == "" {
		base = sanbaoDefaultBaseURL
	}
	base = strings.TrimRight(base, "/")
	for _, suffix := range []string{"/openapi/v1", "/openapi", "/v1"} {
		if strings.HasSuffix(strings.ToLower(base), suffix) {
			base = strings.TrimRight(base[:len(base)-len(suffix)], "/")
			break
		}
	}
	if base == "" {
		base = sanbaoDefaultBaseURL
	}
	return base
}

func (h *OpenAIGatewayHandler) upsertSanbaoVideoTask(ctx context.Context, task *sanbaoVideoTask, request map[string]any, response []byte) error {
	if h.sanbaoDB == nil || task == nil {
		return nil
	}
	reqJSON, _ := json.Marshal(request)
	prompt := strings.TrimSpace(jsonString(request["prompt"]))
	_, err := h.sanbaoDB.ExecContext(ctx, `
		INSERT INTO sanbao_video_tasks
		(task_id, user_id, api_key_id, account_id, group_id, model, upstream_model, prompt, status, ratio, resolution, duration_seconds, video_count, request, response, video_url, download_url, error_message, cost, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,1,$13::jsonb,$14::jsonb,$15,$16,$17,$18,NOW())
		ON CONFLICT (task_id) DO UPDATE SET
			status=EXCLUDED.status, response=EXCLUDED.response, video_url=EXCLUDED.video_url, download_url=EXCLUDED.download_url,
			error_message=EXCLUDED.error_message, cost=CASE WHEN sanbao_video_tasks.cost > 0 THEN sanbao_video_tasks.cost ELSE EXCLUDED.cost END, updated_at=NOW()
	`, task.TaskID, task.UserID, task.APIKeyID, task.AccountID, task.GroupID, task.Model, task.UpstreamModel, prompt, task.Status,
		task.Ratio, task.Resolution, task.DurationSeconds, string(reqJSON), string(response), task.VideoURL, task.DownloadURL, task.ErrorMessage, task.Cost)
	return err
}

func (h *OpenAIGatewayHandler) getSanbaoVideoTask(ctx context.Context, taskID string, userID, apiKeyID int64) (*sanbaoVideoTask, error) {
	if h.sanbaoDB == nil {
		return nil, sql.ErrNoRows
	}
	task := &sanbaoVideoTask{}
	err := h.sanbaoDB.QueryRowContext(ctx, `
		SELECT task_id, user_id, api_key_id, account_id, group_id, model, COALESCE(upstream_model,''), status,
		       COALESCE(ratio,''), COALESCE(resolution,''), duration_seconds, COALESCE(video_url,''), COALESCE(download_url,''),
		       COALESCE(error_message,''), cost, refund_amount, refunded_at, created_at, updated_at
		FROM sanbao_video_tasks
		WHERE task_id=$1 AND user_id=$2 AND api_key_id=$3
	`, taskID, userID, apiKeyID).Scan(&task.TaskID, &task.UserID, &task.APIKeyID, &task.AccountID, &task.GroupID, &task.Model, &task.UpstreamModel,
		&task.Status, &task.Ratio, &task.Resolution, &task.DurationSeconds, &task.VideoURL, &task.DownloadURL, &task.ErrorMessage,
		&task.Cost, &task.RefundAmount, &task.RefundedAt, &task.CreatedAt, &task.UpdatedAt)
	return task, err
}

func updateTaskFromSanbaoResponse(task *sanbaoVideoTask, body []byte) *sanbaoVideoTask {
	clone := *task
	if status := firstGJSON(body, "data.status", "status"); status != "" {
		clone.Status = normalizeSanbaoTaskStatus(status)
	}
	clone.VideoURL = firstNonEmptySanbao(sanbaoVideoURLFromResponse(body), clone.VideoURL)
	clone.DownloadURL = firstNonEmptySanbao(sanbaoDownloadURLFromResponse(body), clone.DownloadURL)
	clone.ErrorMessage = firstNonEmptySanbao(firstGJSON(body, "data.error", "data.message", "error", "message"), clone.ErrorMessage)
	return &clone
}

func (h *OpenAIGatewayHandler) updateSanbaoVideoTaskFromStatus(ctx context.Context, task *sanbaoVideoTask, response []byte) error {
	if h.sanbaoDB == nil || task == nil {
		return nil
	}
	_, err := h.sanbaoDB.ExecContext(ctx, `
		UPDATE sanbao_video_tasks
		SET status=$2, response=$3::jsonb, video_url=$4, download_url=$5, error_message=$6, updated_at=NOW()
		WHERE task_id=$1
	`, task.TaskID, task.Status, string(response), task.VideoURL, task.DownloadURL, task.ErrorMessage)
	return err
}

func (h *OpenAIGatewayHandler) refundSanbaoVideoTask(ctx context.Context, task *sanbaoVideoTask) error {
	if h.sanbaoDB == nil || task == nil || task.Cost <= 0 || task.RefundedAt != nil {
		return nil
	}
	tx, err := h.sanbaoDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, apiKeyID, accountID int64
	var groupID *int64
	var model, resolution string
	var duration int
	var refund float64
	err = tx.QueryRowContext(ctx, `
		UPDATE sanbao_video_tasks
		SET refunded_at=NOW(), refund_amount=cost, updated_at=NOW()
		WHERE task_id=$1 AND refunded_at IS NULL AND cost > 0
		RETURNING user_id, api_key_id, account_id, group_id, model, COALESCE(resolution,''), duration_seconds, cost
	`, task.TaskID).Scan(&userID, &apiKeyID, &accountID, &groupID, &model, &resolution, &duration, &refund)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id=$2`, refund, userID); err != nil {
		return err
	}
	billingMode := string(service.BillingModeVideo)
	requestID := refundRequestID(task.TaskID)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_logs
		(user_id, api_key_id, account_id, request_id, model, requested_model, group_id, total_cost, actual_cost, rate_multiplier,
		 billing_type, request_type, video_count, video_resolution, video_duration_seconds, billing_mode, media_type, created_at)
		VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$7,1,0,1,1,$8,$9,$10,'video',NOW())
		ON CONFLICT (request_id, api_key_id) DO NOTHING
	`, userID, apiKeyID, accountID, requestID, model, groupID, -refund, resolution, duration, billingMode)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if h.billingCacheService != nil {
		h.billingCacheService.InvalidateUserBalance(ctx, userID)
	}
	return nil
}

func (h *OpenAIGatewayHandler) lookupSanbaoUsageCost(ctx context.Context, apiKeyID int64, taskID string) float64 {
	if h.sanbaoDB == nil {
		return 0
	}
	var cost float64
	_ = h.sanbaoDB.QueryRowContext(ctx, `SELECT actual_cost FROM usage_logs WHERE api_key_id=$1 AND request_id=$2 ORDER BY id DESC LIMIT 1`, apiKeyID, taskID).Scan(&cost)
	return cost
}

func sanbaoUsageBillingRequestID(ctx context.Context, upstreamRequestID string) string {
	if ctx != nil {
		if clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
			return "client:" + strings.TrimSpace(clientRequestID)
		}
		if requestID, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
			return "local:" + strings.TrimSpace(requestID)
		}
	}
	if requestID := strings.TrimSpace(upstreamRequestID); requestID != "" {
		return requestID
	}
	return ""
}

func sanbaoVideoURLFromResponse(body []byte) string {
	return firstGJSON(body,
		"data.video_url", "data.videoUrl", "data.url", "data.video", "data.output_url", "data.outputUrl", "data.result", "data.output",
		"video_url", "videoUrl", "url", "video", "output_url", "outputUrl", "result", "output",
	)
}

func sanbaoDownloadURLFromResponse(body []byte) string {
	return firstGJSON(body,
		"data.download_url", "data.downloadUrl", "data.download", "data.content_url", "data.contentUrl",
		"download_url", "downloadUrl", "download", "content_url", "contentUrl",
	)
}

func writeSanbaoUpstreamError(c *gin.Context, statusCode int, headers http.Header, body []byte) {
	for k, values := range headers {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range values {
			c.Writer.Header().Add(k, v)
		}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		c.JSON(statusCode, gin.H{"error": gin.H{"type": "upstream_error", "message": http.StatusText(statusCode)}})
		return
	}
	c.Data(statusCode, "application/json; charset=utf-8", body)
}

func normalizeSanbaoTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case sanbaoVideoStatusSucceeded, "completed", "complete", "success", "successful", "done":
		return sanbaoVideoStatusSucceeded
	case sanbaoVideoStatusFailed, "error", "errored", "cancelled", "canceled":
		return sanbaoVideoStatusFailed
	case sanbaoVideoStatusProcessing, "in_progress", "running", "generating", "working":
		return sanbaoVideoStatusProcessing
	default:
		return sanbaoVideoStatusQueued
	}
}

func normalizeSanbaoDownstreamStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case sanbaoVideoStatusSucceeded, "completed", "success":
		return "completed"
	case sanbaoVideoStatusProcessing, "in_progress", "running":
		return "in_progress"
	case sanbaoVideoStatusFailed, "error":
		return "failed"
	default:
		return "queued"
	}
}

func firstGJSON(body []byte, paths ...string) string {
	for _, path := range paths {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return ""
}

func jsonRawObject(body []byte) any {
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func jsonString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func jsonInt(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		if t > 0 {
			return int(t)
		}
	case json.Number:
		if n, err := strconv.Atoi(t.String()); err == nil && n > 0 {
			return n
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func firstAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstNonEmptySanbao(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func refundRequestID(taskID string) string {
	raw := "refund_" + strings.TrimSpace(taskID)
	if len(raw) <= 64 {
		return raw
	}
	sum := sha1.Sum([]byte(raw))
	return "refund_" + hex.EncodeToString(sum[:])
}
