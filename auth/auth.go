package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Secret is the HMAC secret used to sign tokens. Exported so other packages can reuse it.
var Secret = []byte("replace-with-secure-secret")

// ParseToken validates a JWT token string and returns the "sub" (username) claim.
func ParseToken(tok string) (string, error) {
	if tok == "" {
		return "", errors.New("empty token")
	}
	p := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tok, p, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return Secret, nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", errors.New("invalid token")
	}
	if p.Subject == "" {
		return "", errors.New("missing subject")
	}
	if p.ExpiresAt == nil || time.Now().After(p.ExpiresAt.Time) {
		return "", errors.New("token expired")
	}
	return p.Subject, nil
}
