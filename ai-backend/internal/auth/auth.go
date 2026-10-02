package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const issuer = "mini-redis-ai-backend"

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) (*Tokens, error) {
	if len(secret) < 32 || ttl <= 0 {
		return nil, errors.New("invalid JWT configuration")
	}
	return &Tokens{secret: []byte(secret), ttl: ttl}, nil
}

func (t *Tokens) Issue(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject: userID, Issuer: issuer, Audience: jwt.ClaimStrings{"document-qa"},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *Tokens) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer),
		jwt.WithAudience("document-qa"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !parsed.Valid || claims.Subject == "" {
		return "", domain.ErrUnauthorized
	}
	return claims.Subject, nil
}

func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		return "", domain.ErrInvalid
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(email, "@") {
		return "", domain.ErrInvalid
	}
	return email, nil
}

func HashPassword(password string) (string, error) {
	// bcrypt 最大输入72 bytes；不能静默截断导致两种密码等价。
	if len(password) < 8 || len(password) > 72 {
		return "", domain.ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
