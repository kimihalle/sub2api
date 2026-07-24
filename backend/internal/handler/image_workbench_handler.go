package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// ImageWorkbenchHandler provides a user-facing image generation workbench.
// It deliberately proxies through Sub2API's own /v1 image gateway so existing
// API-key auth, group permissions, scheduling, failover, billing, and usage
// accounting remain the single source of truth.
type ImageWorkbenchHandler struct {
	apiKeyService *service.APIKeyService
	cfg           *config.Config
	store         *imageWorkbenchLogStore
	httpClient    *http.Client
	db            *sql.DB
}

type ImageWorkbenchSubmitRequest struct {
	APIKeyID int64          `json:"api_key_id" binding:"required"`
	Payload  map[string]any `json:"payload" binding:"required"`
}

type ImageWorkbenchPollRequest struct {
	APIKeyID int64 `form:"api_key_id" binding:"required"`
}

type ImageWorkbenchLog struct {
	ID           string         `json:"id"`
	UserID       int64          `json:"user_id"`
	APIKeyID     int64          `json:"api_key_id"`
	APIKeyName   string         `json:"api_key_name,omitempty"`
	AccountID    int64          `json:"account_id,omitempty"`
	AccountName  string         `json:"account_name,omitempty"`
	GroupID      *int64         `json:"group_id,omitempty"`
	GroupName    string         `json:"group_name,omitempty"`
	Model        string         `json:"model,omitempty"`
	Prompt       string         `json:"prompt,omitempty"`
	Status       string         `json:"status"`
	Async        bool           `json:"async"`
	TaskID       string         `json:"task_id,omitempty"`
	ImageURLs    []string       `json:"image_urls,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Request      map[string]any `json:"request,omitempty"`
	Response     map[string]any `json:"response,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type imageWorkbenchUpstreamAccount struct {
	ID           int64
	Name         string
	BaseURL      string
	APIKey       string
	ModelMapping map[string]string
}

type imageWorkbenchLogStore struct {
	mu   sync.Mutex
	path string
}

func NewImageWorkbenchHandler(apiKeyService *service.APIKeyService, cfg *config.Config) *ImageWorkbenchHandler {
	h := &ImageWorkbenchHandler{
		apiKeyService: apiKeyService,
		cfg:           cfg,
		store:         newImageWorkbenchLogStore(cfg),
		httpClient: &http.Client{
			Timeout: 15 * time.Minute,
		},
	}
	if cfg != nil && strings.TrimSpace(cfg.Database.Host) != "" {
		if db, err := sql.Open("postgres", cfg.Database.DSN()); err == nil {
			db.SetMaxOpenConns(4)
			db.SetMaxIdleConns(2)
			db.SetConnMaxLifetime(10 * time.Minute)
			h.db = db
		}
	}
	return h
}

func newImageWorkbenchLogStore(cfg *config.Config) *imageWorkbenchLogStore {
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" && cfg != nil {
		dataDir = strings.TrimSpace(cfg.Pricing.DataDir)
	}
	if dataDir == "" {
		dataDir = "data"
	}
	return &imageWorkbenchLogStore{path: filepath.Join(dataDir, "image_workbench_logs.jsonl")}
}

func (h *ImageWorkbenchHandler) Submit(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	var req ImageWorkbenchSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}
	apiKey, ok := h.requireUsableImageAPIKey(c, subject.UserID, req.APIKeyID)
	if !ok {
		return
	}

	payload := sanitizeImagePayload(req.Payload)
	requestModel, _ := payload["model"].(string)
	prompt, _ := payload["prompt"].(string)
	async := boolFromAny(payload["async"])
	payload["stream"] = false

	logEntry := ImageWorkbenchLog{
		ID:         uuid.NewString(),
		UserID:     subject.UserID,
		APIKeyID:   apiKey.ID,
		APIKeyName: apiKey.Name,
		GroupID:    apiKey.GroupID,
		Model:      strings.TrimSpace(requestModel),
		Prompt:     strings.TrimSpace(prompt),
		Status:     "submitted",
		Async:      async,
		Request:    payload,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if apiKey.Group != nil {
		logEntry.GroupName = apiKey.Group.Name
	}

	body, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image payload"})
		return
	}

	// Go through Sub2API's own OpenAI-compatible image gateway instead of
	// calling the upstream account directly. The gateway is where account
	// scheduling, model mapping, usage records, quota checks, and balance
	// billing are implemented; bypassing it makes workbench generations appear
	// in the workbench log but not in "使用记录".
	statusCode, responseBody, err := h.forwardToSelfGateway(c.Request.Context(), http.MethodPost, "/v1/images/generations", apiKey.Key, body)
	if err != nil {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = err.Error()
		_ = h.store.Upsert(logEntry)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "log_id": logEntry.ID})
		return
	}

	parsedResponse := map[string]any{}
	_ = json.Unmarshal(responseBody, &parsedResponse)
	logEntry.Response = parsedResponse
	upstreamImageURLs := extractImageURLs(parsedResponse)
	logEntry.ImageURLs = proxyImageURLs(logEntry.ID, upstreamImageURLs)
	logEntry.TaskID = extractTaskID(parsedResponse)
	if statusCode >= 200 && statusCode < 300 {
		if async && logEntry.TaskID != "" && len(upstreamImageURLs) == 0 {
			logEntry.Status = firstNonEmptyString(statusString(parsedResponse), "queued")
		} else {
			logEntry.Status = firstNonEmptyString(statusString(parsedResponse), "completed")
		}
	} else {
		logEntry.Status = "failed"
		logEntry.ErrorMessage = extractErrorMessage(parsedResponse)
	}
	logEntry.UpdatedAt = time.Now()
	_ = h.store.Upsert(logEntry)

	c.Data(statusCode, contentTypeJSON, responseBodyWithLogIDAndProxiedImages(responseBody, logEntry.ID))
}

func (h *ImageWorkbenchHandler) PollTask(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id is required"})
		return
	}
	var req ImageWorkbenchPollRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key_id is required"})
		return
	}
	apiKey, ok := h.requireUsableImageAPIKey(c, subject.UserID, req.APIKeyID)
	if !ok {
		return
	}
	existingEntry, existingEntryFound, _ := h.store.FindTask(subject.UserID, taskID)

	upstreamAccount, err := h.selectImageUpstreamAccountForTask(c.Request.Context(), subject.UserID, apiKey.GroupID, taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	statusCode, responseBody, err := h.forwardToUpstream(c.Request.Context(), http.MethodGet, upstreamAccount, "/images/generations/"+taskID, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	parsedResponse := map[string]any{}
	_ = json.Unmarshal(responseBody, &parsedResponse)
	upstreamImageURLs := extractImageURLs(parsedResponse)
	logID := ""
	if existingEntryFound {
		logID = existingEntry.ID
	}
	_ = h.store.UpdateTask(subject.UserID, taskID, func(entry *ImageWorkbenchLog) {
		logID = entry.ID
		entry.Status = firstNonEmptyString(statusString(parsedResponse), entry.Status)
		entry.ImageURLs = proxyImageURLs(entry.ID, upstreamImageURLs)
		entry.Response = parsedResponse
		if statusCode >= 400 {
			entry.Status = "failed"
			entry.ErrorMessage = extractErrorMessage(parsedResponse)
		}
		entry.UpdatedAt = time.Now()
	})
	if logID != "" {
		responseBody = responseBodyWithLogIDAndProxiedImages(responseBody, logID)
	}

	c.Data(statusCode, contentTypeJSON, responseBody)
}

func (h *ImageWorkbenchHandler) Logs(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parsePositiveInt(c.DefaultQuery("page_size", "20"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	status := strings.TrimSpace(c.Query("status"))
	model := strings.TrimSpace(c.Query("model"))

	logs, err := h.store.List(subject.UserID, status, model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range logs {
		logs[i].ImageURLs = proxyImageURLs(logs[i].ID, preferredImageURLs(logs[i]))
	}
	total := len(logs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       logs[start:end],
		"pagination": gin.H{"page": page, "page_size": pageSize, "total": total},
	})
}

func (h *ImageWorkbenchHandler) ImageContent(c *gin.Context) {
	logID := strings.TrimSpace(c.Param("log_id"))
	index := parsePositiveInt(c.Param("index"), 0)
	if logID == "" || index < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image path"})
		return
	}
	entry, ok, err := h.store.FindByID(logID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	upstreamURLs := extractImageURLs(entry.Response)
	if len(upstreamURLs) == 0 {
		upstreamURLs = entry.ImageURLs
	}
	if index >= len(upstreamURLs) {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	}
	upstreamURL := strings.TrimSpace(upstreamURLs[index])
	if !isHTTPURL(upstreamURL) || strings.Contains(upstreamURL, "/api/v1/image-workbench/images/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "image source not available"})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image source"})
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch image failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "fetch image failed"})
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("Content-Type", contentType)
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		c.Header("Content-Length", contentLength)
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func (h *ImageWorkbenchHandler) requireUsableImageAPIKey(c *gin.Context, userID, apiKeyID int64) (*service.APIKey, bool) {
	if h == nil || h.apiKeyService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "image workbench is not configured"})
		return nil, false
	}
	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return nil, false
	}
	if !apiKey.IsActive() || apiKey.IsExpired() || apiKey.IsQuotaExhausted() {
		c.JSON(http.StatusForbidden, gin.H{"error": "API key is not usable"})
		return nil, false
	}
	if apiKey.Group == nil || !service.GroupAllowsImageGeneration(apiKey.Group) {
		c.JSON(http.StatusForbidden, gin.H{"error": service.ImageGenerationPermissionMessage()})
		return nil, false
	}
	if apiKey.Group.Platform != service.PlatformOpenAI && apiKey.Group.Platform != service.PlatformComposite && apiKey.Group.Platform != service.PlatformGrok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected API key group is not image endpoint compatible"})
		return nil, false
	}
	return apiKey, true
}

func (h *ImageWorkbenchHandler) selectImageUpstreamAccount(ctx context.Context, groupID *int64, requestedModel string) (*imageWorkbenchUpstreamAccount, string, error) {
	if h == nil || h.db == nil {
		return nil, "", errors.New("生图账号读取失败：数据库未初始化")
	}
	if groupID == nil || *groupID <= 0 {
		return nil, "", errors.New("请选择已绑定生图分组的 API Key")
	}
	rows, err := h.db.QueryContext(ctx, `
SELECT a.id, a.name, a.credentials
FROM accounts a
JOIN account_groups ag ON ag.account_id = a.id
WHERE ag.group_id = $1
  AND a.deleted_at IS NULL
  AND a.status = 'active'
  AND a.schedulable = TRUE
  AND a.type = 'apikey'
  AND a.platform = 'openai'
ORDER BY ag.priority ASC, a.priority ASC, a.id ASC`, *groupID)
	if err != nil {
		return nil, "", fmt.Errorf("读取生图账号失败：%w", err)
	}
	defer rows.Close()

	for rows.Next() {
		account, err := scanImageWorkbenchAccount(rows)
		if err != nil || account == nil {
			continue
		}
		if strings.TrimSpace(account.BaseURL) == "" || strings.TrimSpace(account.APIKey) == "" {
			continue
		}
		mappedModel, ok := account.resolveModel(requestedModel)
		if ok {
			return account, mappedModel, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("读取生图账号失败：%w", err)
	}
	return nil, "", fmt.Errorf("没有可用的生图账号支持模型：%s", requestedModel)
}

func (h *ImageWorkbenchHandler) selectImageUpstreamAccountByID(ctx context.Context, groupID *int64, accountID int64) (*imageWorkbenchUpstreamAccount, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("生图账号读取失败：数据库未初始化")
	}
	if groupID == nil || *groupID <= 0 || accountID <= 0 {
		return nil, errors.New("生图账号不存在")
	}
	row := h.db.QueryRowContext(ctx, `
SELECT a.id, a.name, a.credentials
FROM accounts a
JOIN account_groups ag ON ag.account_id = a.id
WHERE ag.group_id = $1
  AND a.id = $2
  AND a.deleted_at IS NULL
  AND a.status = 'active'
  AND a.schedulable = TRUE
  AND a.type = 'apikey'
  AND a.platform = 'openai'`, *groupID, accountID)
	account, err := scanImageWorkbenchAccount(row)
	if err != nil {
		return nil, fmt.Errorf("生图账号不存在或不可用：%w", err)
	}
	return account, nil
}

func (h *ImageWorkbenchHandler) selectImageUpstreamAccountForTask(ctx context.Context, userID int64, groupID *int64, taskID string) (*imageWorkbenchUpstreamAccount, error) {
	if entry, ok, _ := h.store.FindTask(userID, taskID); ok && entry.AccountID > 0 {
		if account, err := h.selectImageUpstreamAccountByID(ctx, groupID, entry.AccountID); err == nil {
			return account, nil
		}
	}
	account, _, err := h.selectImageUpstreamAccount(ctx, groupID, "")
	return account, err
}

type imageWorkbenchAccountScanner interface {
	Scan(dest ...any) error
}

func scanImageWorkbenchAccount(scanner imageWorkbenchAccountScanner) (*imageWorkbenchUpstreamAccount, error) {
	var account imageWorkbenchUpstreamAccount
	var credentialsRaw []byte
	if err := scanner.Scan(&account.ID, &account.Name, &credentialsRaw); err != nil {
		return nil, err
	}
	var credentials map[string]any
	if err := json.Unmarshal(credentialsRaw, &credentials); err != nil {
		return nil, err
	}
	account.BaseURL = strings.TrimSpace(firstNonEmptyString(anyString(credentials["base_url"]), anyString(credentials["baseURL"])))
	account.APIKey = strings.TrimSpace(anyString(credentials["api_key"]))
	account.ModelMapping = stringMapFromAny(credentials["model_mapping"])
	return &account, nil
}

func (a *imageWorkbenchUpstreamAccount) resolveModel(requestedModel string) (string, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return "", true
	}
	if a == nil || len(a.ModelMapping) == 0 {
		return requestedModel, true
	}
	if mapped, ok := a.ModelMapping[requestedModel]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped), true
	}
	return "", false
}

func (h *ImageWorkbenchHandler) forwardToUpstream(ctx context.Context, method string, account *imageWorkbenchUpstreamAccount, path string, body []byte) (int, []byte, error) {
	if account == nil {
		return 0, nil, errors.New("生图账号不存在")
	}
	url := buildImageWorkbenchUpstreamURL(account.BaseURL, path)
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+account.APIKey)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

func buildImageWorkbenchUpstreamURL(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = "/" + strings.TrimLeft(path, "/")
	return baseURL + path
}

func (h *ImageWorkbenchHandler) forwardToSelfGateway(ctx context.Context, method, path, apiKey string, body []byte) (int, []byte, error) {
	port := 8080
	if h != nil && h.cfg != nil && h.cfg.Server.Port > 0 {
		port = h.cfg.Server.Port
	}
	// Use IPv6 loopback instead of 127.0.0.1 here. On Windows it is possible
	// for another local process to bind 127.0.0.1:port while Sub2API is
	// listening on [::]:port; using 127.0.0.1 would silently send the workbench
	// request to that other process and surface confusing upstream errors.
	url := fmt.Sprintf("http://[::1]:%d%s", port, path)
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

func sanitizeImagePayload(in map[string]any) map[string]any {
	allowed := map[string]struct{}{
		"model": {}, "prompt": {}, "aspect_ratio": {}, "output_resolution": {}, "image_size": {},
		"size": {}, "quality": {}, "n": {}, "async": {}, "stream": {}, "response_format": {},
		"image": {}, "images": {}, "imageUrls": {}, "image_urls": {}, "reference_images": {},
		"referenceImages": {}, "image_refs": {}, "background": {}, "moderation": {}, "style": {},
	}
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		if _, ok := allowed[k]; ok {
			out[k] = v
		}
	}
	return out
}

const contentTypeJSON = "application/json; charset=utf-8"

func responseBodyWithLogID(body []byte, logID string) []byte {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	m["sub2api_image_log_id"] = logID
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

func responseBodyWithLogIDAndProxiedImages(body []byte, logID string) []byte {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	m["sub2api_image_log_id"] = logID
	if data, ok := m["data"].([]any); ok {
		for i, item := range data {
			obj, _ := item.(map[string]any)
			if obj == nil {
				continue
			}
			rawURL, _ := obj["url"].(string)
			if isHTTPURL(rawURL) {
				obj["url"] = proxyImageURL(logID, i)
			}
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

func preferredImageURLs(entry ImageWorkbenchLog) []string {
	if urls := extractImageURLs(entry.Response); len(urls) > 0 {
		return urls
	}
	return entry.ImageURLs
}

func proxyImageURLs(logID string, upstreamURLs []string) []string {
	if len(upstreamURLs) == 0 {
		return nil
	}
	out := make([]string, 0, len(upstreamURLs))
	for i, rawURL := range upstreamURLs {
		if isHTTPURL(rawURL) && !strings.Contains(rawURL, "/api/v1/image-workbench/images/") {
			out = append(out, proxyImageURL(logID, i))
			continue
		}
		out = append(out, rawURL)
	}
	return out
}

func proxyImageURL(logID string, index int) string {
	return "/api/v1/image-workbench/images/" + url.PathEscape(logID) + "/" + strconv.Itoa(index)
}

func isHTTPURL(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func extractImageURLs(m map[string]any) []string {
	var urls []string
	if data, ok := m["data"].([]any); ok {
		for _, item := range data {
			obj, _ := item.(map[string]any)
			if url, _ := obj["url"].(string); strings.TrimSpace(url) != "" {
				urls = append(urls, url)
			}
			if b64, _ := obj["b64_json"].(string); strings.TrimSpace(b64) != "" {
				urls = append(urls, "data:image/png;base64,"+b64)
			}
		}
	}
	return urls
}

func extractTaskID(m map[string]any) string {
	for _, key := range []string{"task_id", "id"} {
		if value, _ := m[key].(string); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func statusString(m map[string]any) string {
	if value, _ := m["status"].(string); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func extractErrorMessage(m map[string]any) string {
	if errObj, _ := m["error"].(map[string]any); errObj != nil {
		if msg, _ := errObj["message"].(string); msg != "" {
			return msg
		}
	}
	if msg, _ := m["message"].(string); msg != "" {
		return msg
	}
	return "upstream request failed"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func stringMapFromAny(v any) map[string]string {
	out := map[string]string{}
	switch m := v.(type) {
	case map[string]any:
		for k, raw := range m {
			if s, ok := raw.(string); ok && strings.TrimSpace(k) != "" && strings.TrimSpace(s) != "" {
				out[strings.TrimSpace(k)] = strings.TrimSpace(s)
			}
		}
	case map[string]string:
		for k, s := range m {
			if strings.TrimSpace(k) != "" && strings.TrimSpace(s) != "" {
				out[strings.TrimSpace(k)] = strings.TrimSpace(s)
			}
		}
	}
	return out
}

func boolFromAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true") || x == "1"
	default:
		return false
	}
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func (s *imageWorkbenchLogStore) Upsert(entry ImageWorkbenchLog) error {
	if s == nil {
		return errors.New("log store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readLocked()
	if err != nil {
		return err
	}
	replaced := false
	for i := range logs {
		if logs[i].ID == entry.ID {
			logs[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		logs = append(logs, entry)
	}
	return s.writeLocked(logs)
}

func (s *imageWorkbenchLogStore) UpdateTask(userID int64, taskID string, fn func(*ImageWorkbenchLog)) error {
	if s == nil || taskID == "" || fn == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readLocked()
	if err != nil {
		return err
	}
	changed := false
	for i := range logs {
		if logs[i].UserID == userID && logs[i].TaskID == taskID {
			fn(&logs[i])
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.writeLocked(logs)
}

func (s *imageWorkbenchLogStore) FindTask(userID int64, taskID string) (ImageWorkbenchLog, bool, error) {
	if s == nil || taskID == "" {
		return ImageWorkbenchLog{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readLocked()
	if err != nil {
		return ImageWorkbenchLog{}, false, err
	}
	for i := range logs {
		if logs[i].UserID == userID && logs[i].TaskID == taskID {
			return logs[i], true, nil
		}
	}
	return ImageWorkbenchLog{}, false, nil
}

func (s *imageWorkbenchLogStore) FindByID(logID string) (ImageWorkbenchLog, bool, error) {
	if s == nil || strings.TrimSpace(logID) == "" {
		return ImageWorkbenchLog{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readLocked()
	if err != nil {
		return ImageWorkbenchLog{}, false, err
	}
	for i := range logs {
		if logs[i].ID == logID {
			return logs[i], true, nil
		}
	}
	return ImageWorkbenchLog{}, false, nil
}

func (s *imageWorkbenchLogStore) List(userID int64, status, model string) ([]ImageWorkbenchLog, error) {
	if s == nil {
		return nil, errors.New("log store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	out := make([]ImageWorkbenchLog, 0, len(logs))
	for _, entry := range logs {
		if entry.UserID != userID {
			continue
		}
		if status != "" && entry.Status != status {
			continue
		}
		if model != "" && !strings.Contains(strings.ToLower(entry.Model), strings.ToLower(model)) {
			continue
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *imageWorkbenchLogStore) readLocked() ([]ImageWorkbenchLog, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ImageWorkbenchLog{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	logs := make([]ImageWorkbenchLog, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry ImageWorkbenchLog
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			logs = append(logs, entry)
		}
	}
	return logs, nil
}

func (s *imageWorkbenchLogStore) writeLocked(logs []ImageWorkbenchLog) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, entry := range logs {
		if err := enc.Encode(entry); err != nil {
			return err
		}
	}
	return os.WriteFile(s.path, buf.Bytes(), 0o600)
}
