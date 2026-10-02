package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/ai-backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestEmailAndPasswordValidation(t *testing.T) {
	for _, test := range []struct {
		input, want string
		valid       bool
	}{
		{"  ALICE@example.com  ", "alice@example.com", true},
		{"Alice <alice@example.com>", "", false},
		{"missing-at", "", false}, {"", "", false},
	} {
		got, err := NormalizeEmail(test.input)
		if got != test.want || (err == nil) != test.valid {
			t.Fatalf("%q=%q/%v", test.input, got, err)
		}
	}
	for _, password := range []string{"short", strings.Repeat("x", 73)} {
		if _, err := HashPassword(password); !errors.Is(err, domain.ErrInvalid) {
			t.Fatal("invalid password accepted")
		}
	}
	hash, err := HashPassword("valid-password")
	if err != nil || hash == "valid-password" || !VerifyPassword(hash, "valid-password") || VerifyPassword(hash, "wrong-password") {
		t.Fatal("bcrypt contract failed")
	}
}

func TestJWTRejectsAlgorithmIssuerAudienceExpiryAndTampering(t *testing.T) {
	secret := strings.Repeat("s", 32)
	tokens, _ := NewTokens(secret, time.Hour)
	valid, err := tokens.Issue("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := tokens.Parse(valid); id != "user-a" || err != nil {
		t.Fatalf("parse=%q/%v", id, err)
	}
	for _, name := range []string{"algorithm", "issuer", "audience", "expired", "missing-expiry", "empty-subject"} {
		t.Run(name, func(t *testing.T) {
			claims := jwt.RegisteredClaims{Subject: "user-a", Issuer: issuer, Audience: jwt.ClaimStrings{"document-qa"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
			method := jwt.SigningMethodHS256
			switch name {
			case "algorithm":
				method = jwt.SigningMethodHS384
			case "issuer":
				claims.Issuer = "elsewhere"
			case "audience":
				claims.Audience = jwt.ClaimStrings{"elsewhere"}
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
			case "missing-expiry":
				claims.ExpiresAt = nil
			case "empty-subject":
				claims.Subject = ""
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Parse(token); !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatal("invalid JWT accepted")
			}
		})
	}
	parts := strings.Split(valid, ".")
	parts[1] = "e30"
	if _, err := tokens.Parse(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered token accepted")
	}
}
