package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/auth"
	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
)

type stubRepo struct {
	Repository
	err         error
	panicOnRead bool
}

func (s stubRepo) Conversation(_ context.Context, _, _ string) (domain.Conversation, error) {
	if s.panicOnRead {
		panic("must not escape")
	}
	return domain.Conversation{}, s.err
}

func (s stubRepo) Conversations(_ context.Context, _ string, _, _ int) ([]domain.Conversation, error) {
	return []domain.Conversation{}, s.err
}

func testAPI(t *testing.T, repo Repository) (*API, string) {
	t.Helper()
	tokens, err := auth.NewTokens(strings.Repeat("s", 32), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue("user-a")
	if err != nil {
		t.Fatal(err)
	}
	return &API{Repo: repo, Tokens: tokens, Quota: 100, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}, token
}

func TestErrorsAuthenticationPaginationAndRequestID(t *testing.T) {
	for _, test := range []struct {
		name, path, token string
		err               error
		status            int
		code              string
	}{
		{"missing token", "/api/v1/conversations", "", nil, 401, "UNAUTHORIZED"},
		{"bad token", "/api/v1/conversations", "invalid", nil, 401, "UNAUTHORIZED"},
		{"wrong owner", "/api/v1/conversations/id", "valid", domain.ErrForbidden, 403, "FORBIDDEN"},
		{"missing", "/api/v1/conversations/id", "valid", domain.ErrNotFound, 404, "NOT_FOUND"},
		{"private error", "/api/v1/conversations/id", "valid", errors.New("db-password=/private/path"), 500, "INTERNAL"},
		{"page zero", "/api/v1/conversations?page=0", "valid", nil, 400, "INVALID_REQUEST"},
		{"oversize", "/api/v1/conversations?page_size=101", "valid", nil, 400, "INVALID_REQUEST"},
		{"invalid page", "/api/v1/conversations?page=nan", "valid", nil, 400, "INVALID_REQUEST"},
		{"valid", "/api/v1/conversations?page=2&page_size=2", "valid", nil, 200, "OK"},
	} {
		t.Run(test.name, func(t *testing.T) {
			api, token := testAPI(t, stubRepo{err: test.err})
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.token == "valid" {
				request.Header.Set("Authorization", "Bearer "+token)
			} else if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			request.Header.Set("X-Request-ID", "test-id")
			response := httptest.NewRecorder()
			api.Router().ServeHTTP(response, request)
			body := response.Body.String()
			if response.Code != test.status || !strings.Contains(body, test.code) || strings.Contains(body, "db-password") || response.Header().Get("X-Request-ID") != "test-id" {
				t.Fatalf("status=%d body=%s headers=%v", response.Code, body, response.Header())
			}
		})
	}
}

func TestRecoverAndUntrustedRequestID(t *testing.T) {
	api, token := testAPI(t, stubRepo{panicOnRead: true})
	request := httptest.NewRequest("GET", "/api/v1/conversations/id", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Request-ID", "invalid id")
	response := httptest.NewRecorder()
	api.Router().ServeHTTP(response, request)
	if response.Code != 500 || strings.Contains(response.Body.String(), "must not escape") || response.Header().Get("X-Request-ID") == "invalid id" {
		t.Fatalf("panic/ID failure: %d %s", response.Code, response.Body)
	}
}

func TestJSONInputBudgetsAndTrailingObjects(t *testing.T) {
	for _, body := range []string{`{"title":"x"}{"title":"y"}`, `{"title":"x","unknown":1}`, `{"title":""}`, `{"title":"` + strings.Repeat("x", 1<<20) + `"}`} {
		api, token := testAPI(t, stubRepo{})
		request := httptest.NewRequest("POST", "/api/v1/conversations", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		api.Router().ServeHTTP(response, request)
		if response.Code != 400 {
			t.Fatalf("bad input status=%d", response.Code)
		}
	}
}
