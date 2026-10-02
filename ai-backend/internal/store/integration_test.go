package store

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/auth"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/httpapi"
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("real MySQL integration requires TEST_DATABASE_DSN; unit tests do not substitute")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, err := Open(ctx, dsn, 10, 5, time.Minute, 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMySQLRegisterRollbackPaginationAndOwnerIsolation(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	email := "rollback-" + domain.NewID() + "@example.test"
	if _, err := s.CreateUser(ctx, email, "unused-hash", -1); err == nil {
		t.Fatal("constraint failure fixture did not fail")
	}
	if _, err := s.UserByEmail(ctx, email); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("transaction left a partial user: %v", err)
	}
	hash, _ := auth.HashPassword("test-password")
	user, err := s.CreateUser(ctx, email, hash, 100)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UserByEmail(ctx, email)
	if err != nil || !auth.VerifyPassword(got.PasswordHash, "test-password") || got.PasswordHash == "test-password" {
		t.Fatal("bcrypt persistence failed")
	}
	if _, err := s.CreateUser(ctx, email, hash, 100); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate=%v", err)
	}
	other, err := s.CreateUser(ctx, domain.NewID()+"@example.test", hash, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := s.CreateConversation(ctx, user.ID, "conversation"); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Conversations(ctx, user.ID, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Conversations(ctx, user.ID, 2, 2)
	if err != nil || len(first) != 2 || len(second) != 2 {
		t.Fatalf("pages=%v/%v err=%v", first, second, err)
	}
	seen := map[string]bool{}
	for _, item := range first {
		seen[item.ID] = true
	}
	for _, item := range second {
		if seen[item.ID] {
			t.Fatal("second page overlaps")
		}
	}
	if _, err := s.Conversation(ctx, first[0].ID, other.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("foreign owner=%v", err)
	}
	if _, err := s.RenameConversation(ctx, first[0].ID, other.ID, "stolen"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("foreign rename accepted")
	}
	if err := s.DeleteConversation(ctx, first[0].ID, other.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("foreign delete accepted")
	}
	if _, err := s.RenameConversation(ctx, first[0].ID, user.ID, "new-title"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteConversation(ctx, first[0].ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Conversation(ctx, first[0].ID, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("delete failed")
	}
	t.Log("real MySQL: register/quota rollback, bcrypt, uniqueness, stable page2, owner read/write/delete passed")
}

func TestMySQLHTTPRegisterLoginJWTAndForbidden(t *testing.T) {
	s := integrationStore(t)
	tokens, _ := auth.NewTokens(strings.Repeat("s", 32), time.Hour)
	api := &httpapi.API{Repo: s, Tokens: tokens, Quota: 100, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Ready: s.DB.PingContext}
	router := api.Router()
	do := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	email := domain.NewID() + "@example.test"
	credentials := `{"email":"` + email + `","password":"test-password"}`
	if res := do("POST", "/api/v1/auth/register", credentials, ""); res.Code != 201 {
		t.Fatalf("register=%d %s", res.Code, res.Body)
	}
	res := do("POST", "/api/v1/auth/login", credentials, "")
	if res.Code != 200 {
		t.Fatalf("login=%d %s", res.Code, res.Body)
	}
	var login struct {
		Data struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	res = do("POST", "/api/v1/conversations", `{"title":"private"}`, login.Data.Token)
	var conversation struct {
		Data domain.Conversation `json:"data"`
	}
	if res.Code != 201 || json.Unmarshal(res.Body.Bytes(), &conversation) != nil {
		t.Fatalf("create=%d", res.Code)
	}
	other, _ := s.CreateUser(context.Background(), domain.NewID()+"@example.test", "unused", 100)
	otherToken, _ := tokens.Issue(other.ID)
	if res := do("GET", "/api/v1/conversations/"+conversation.Data.ID, "", otherToken); res.Code != 403 {
		t.Fatalf("foreign read=%d", res.Code)
	}
	if res := do("GET", "/api/v1/conversations", "", "broken"); res.Code != http.StatusUnauthorized {
		t.Fatal("bad token not 401")
	}
	if res := do("GET", "/health/ready", "", ""); res.Code != 200 {
		t.Fatal("readiness failed")
	}
}

func TestMySQLConversationTimestampsStayUTCInNonUTCSession(t *testing.T) {
	s := integrationStore(t)
	s.DB.SetMaxOpenConns(1)
	s.DB.SetMaxIdleConns(1)
	ctx := context.Background()
	// 只改本测试持有的独占连接，模拟常见+08:00服务器会话，不改global。
	if _, err := s.DB.ExecContext(ctx, "SET SESSION time_zone='+08:00'"); err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(ctx, domain.NewID()+"@example.test", "fixture-hash", 100)
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.CreateConversation(ctx, user.ID, "before")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.RenameConversation(ctx, item.ID, user.ID, "after")
	if err != nil {
		t.Fatal(err)
	}
	if difference := time.Since(updated.UpdatedAt); difference < -time.Minute || difference > time.Minute {
		t.Fatalf("UpdatedAt is not UTC: decoded=%s difference=%s", updated.UpdatedAt, difference)
	}
}

func TestMySQLNewConnectionsUseUTC(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("requires owned real MySQL fixture")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn+"&time_zone=%27%2B08%3A00%27", 2, 1, time.Minute, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	var zone string
	if err := s.DB.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&zone); err != nil {
		t.Fatal(err)
	}
	if zone != "+00:00" {
		t.Fatalf("driver connection timezone=%q", zone)
	}
}
