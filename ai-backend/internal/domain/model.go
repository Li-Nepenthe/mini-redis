package domain

import (
	"crypto/rand"
	"errors"
	"time"
)

var (
	ErrInvalid      = errors.New("invalid request")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrQuota        = errors.New("quota exceeded")
	ErrRateLimited  = errors.New("rate limited")
	ErrUnavailable  = errors.New("unavailable")
)

func NewID() string { return rand.Text() }

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Citation struct {
	DocumentID string `json:"document_id"`
	Name       string `json:"name"`
	Paragraph  int    `json:"paragraph"`
}

type Message struct {
	ID               string     `json:"id"`
	ConversationID   string     `json:"conversation_id"`
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	PromptTokens     int64      `json:"prompt_tokens"`
	CompletionTokens int64      `json:"completion_tokens"`
	Citations        []Citation `json:"citations"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Usage struct {
	Month      string `json:"month"`
	TokenUsed  int64  `json:"token_used"`
	TokenLimit int64  `json:"token_limit"`
}
