package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/auth"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
	"github.com/gin-gonic/gin"
)

type Repository interface {
	CreateUser(context.Context, string, string, int64) (domain.User, error)
	UserByEmail(context.Context, string) (domain.User, error)
	CreateConversation(context.Context, string, string) (domain.Conversation, error)
	Conversation(context.Context, string, string) (domain.Conversation, error)
	Conversations(context.Context, string, int, int) ([]domain.Conversation, error)
	RenameConversation(context.Context, string, string, string) (domain.Conversation, error)
	DeleteConversation(context.Context, string, string) error
	Usage(context.Context, string) (domain.Usage, error)
}

type API struct {
	Repo   Repository
	Tokens *auth.Tokens
	Quota  int64
	Logger *slog.Logger
	Ready  func(context.Context) error
}

func (a *API) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	// Gin默认恢复日志含原始panic/请求；只保留结构化分类，避免泄漏凭据或正文。
	router.Use(a.requestID(), gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		a.Logger.Error("request panic", "request_id", requestID(c))
		a.fail(c, errors.New("internal failure"))
	}))
	router.NoRoute(func(c *gin.Context) { a.fail(c, domain.ErrNotFound) })
	router.NoMethod(func(c *gin.Context) { a.fail(c, domain.ErrInvalid) })
	router.HandleMethodNotAllowed = true
	router.GET("/health/live", func(c *gin.Context) { a.ok(c, http.StatusOK, gin.H{"status": "live"}) })
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if a.Ready != nil && a.Ready(ctx) != nil {
			a.fail(c, domain.ErrUnavailable)
			return
		}
		a.ok(c, http.StatusOK, gin.H{"status": "ready"})
	})
	v1 := router.Group("/api/v1")
	v1.POST("/auth/register", a.register)
	v1.POST("/auth/login", a.login)
	protected := v1.Group("")
	protected.Use(a.authorize())
	protected.POST("/conversations", a.createConversation)
	protected.GET("/conversations", a.conversations)
	protected.GET("/conversations/:id", a.conversation)
	// 原API表省略PATCH/DELETE，但M1明确要求CRUD，补齐这两个资源操作。
	protected.PATCH("/conversations/:id", a.renameConversation)
	protected.DELETE("/conversations/:id", a.deleteConversation)
	protected.GET("/usage", a.usage)
	return router
}

func (a *API) requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		valid := len(id) > 0 && len(id) <= 64
		for _, r := range id {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				valid = false
			}
		}
		if !valid {
			id = domain.NewID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		started := time.Now()
		c.Next()
		a.Logger.Info("http request", "request_id", id, "method", c.Request.Method,
			"path", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
	}
}

func requestID(c *gin.Context) string { return c.GetString("request_id") }
func owner(c *gin.Context) string     { return c.GetString("owner") }

func (a *API) authorize() gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Fields(c.GetHeader("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			a.fail(c, domain.ErrUnauthorized)
			return
		}
		id, err := a.Tokens.Parse(fields[1])
		if err != nil {
			a.fail(c, domain.ErrUnauthorized)
			return
		}
		c.Set("owner", id)
		c.Next()
	}
}

func bind(c *gin.Context, target any) error {
	// 只对JSON业务请求限1MiB；后续文档上传有独立10MiB预算。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.ErrInvalid
	}
	return nil
}

func (a *API) ok(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"code": "OK", "data": data, "request_id": requestID(c)})
}

func (a *API) fail(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "INTERNAL", "internal server error"
	for _, mapping := range []struct {
		err           error
		status        int
		code, message string
	}{
		{domain.ErrInvalid, 400, "INVALID_REQUEST", "invalid request"},
		{domain.ErrUnauthorized, 401, "UNAUTHORIZED", "authentication required"},
		{domain.ErrForbidden, 403, "FORBIDDEN", "resource is owned by another user"},
		{domain.ErrNotFound, 404, "NOT_FOUND", "resource not found"},
		{domain.ErrConflict, 409, "CONFLICT", "resource already exists"},
		{domain.ErrQuota, 429, "QUOTA_EXCEEDED", "monthly token quota exceeded"},
		{domain.ErrRateLimited, 429, "RATE_LIMITED", "request rate exceeded"},
		{domain.ErrUnavailable, 503, "UNAVAILABLE", "service temporarily unavailable"},
	} {
		if errors.Is(err, mapping.err) {
			status, code, message = mapping.status, mapping.code, mapping.message
			break
		}
	}
	if status == 500 {
		// 不回写SQL/路径/提供商错误；日志只保留安全分类与request_id。
		a.Logger.Error("request failed", "request_id", requestID(c), "category", code)
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": message, "request_id": requestID(c)})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) register(c *gin.Context) {
	var input credentials
	if err := bind(c, &input); err != nil {
		a.fail(c, err)
		return
	}
	email, err := auth.NormalizeEmail(input.Email)
	if err != nil {
		a.fail(c, err)
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		a.fail(c, err)
		return
	}
	user, err := a.Repo.CreateUser(c.Request.Context(), email, hash, a.Quota)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 201, user)
}

func (a *API) login(c *gin.Context) {
	var input credentials
	if err := bind(c, &input); err != nil {
		a.fail(c, err)
		return
	}
	email, err := auth.NormalizeEmail(input.Email)
	if err != nil || len(input.Password) > 72 {
		a.fail(c, domain.ErrUnauthorized)
		return
	}
	user, err := a.Repo.UserByEmail(c.Request.Context(), email)
	if err != nil || user.Status != "active" || !auth.VerifyPassword(user.PasswordHash, input.Password) {
		a.fail(c, domain.ErrUnauthorized)
		return
	}
	token, err := a.Tokens.Issue(user.ID)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, gin.H{"access_token": token, "token_type": "Bearer"})
}

func title(c *gin.Context) (string, error) {
	var input struct {
		Title string `json:"title"`
	}
	if err := bind(c, &input); err != nil {
		return "", err
	}
	value := strings.TrimSpace(input.Title)
	if value == "" || utf8.RuneCountInString(value) > 200 {
		return "", domain.ErrInvalid
	}
	return value, nil
}

func (a *API) createConversation(c *gin.Context) {
	value, err := title(c)
	if err != nil {
		a.fail(c, err)
		return
	}
	item, err := a.Repo.CreateConversation(c.Request.Context(), owner(c), value)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 201, item)
}

func pagination(c *gin.Context) (int, int, error) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 10000 {
		return 0, 0, domain.ErrInvalid
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || size < 1 || size > 100 {
		return 0, 0, domain.ErrInvalid
	}
	return page, size, nil
}

func (a *API) conversations(c *gin.Context) {
	page, size, err := pagination(c)
	if err != nil {
		a.fail(c, err)
		return
	}
	items, err := a.Repo.Conversations(c.Request.Context(), owner(c), page, size)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, gin.H{"items": items, "page": page, "page_size": size})
}

func (a *API) conversation(c *gin.Context) {
	item, err := a.Repo.Conversation(c.Request.Context(), c.Param("id"), owner(c))
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, item)
}

func (a *API) renameConversation(c *gin.Context) {
	value, err := title(c)
	if err != nil {
		a.fail(c, err)
		return
	}
	item, err := a.Repo.RenameConversation(c.Request.Context(), c.Param("id"), owner(c), value)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, item)
}

func (a *API) deleteConversation(c *gin.Context) {
	if err := a.Repo.DeleteConversation(c.Request.Context(), c.Param("id"), owner(c)); err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, gin.H{"deleted": true})
}

func (a *API) usage(c *gin.Context) {
	usage, err := a.Repo.Usage(c.Request.Context(), owner(c))
	if err != nil {
		a.fail(c, err)
		return
	}
	a.ok(c, 200, usage)
}
