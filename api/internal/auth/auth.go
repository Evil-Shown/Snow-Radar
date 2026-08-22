// Package auth: Argon2id password hashing + RS256 JWT session tokens.
// Boring, proven choices per SECURITY.md; no custom crypto.
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/golang-jwt/jwt/v5"
)

// ---- Argon2id password hashing ----

const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

var ErrInvalidCredentials = errors.New("auth: invalid credentials")

type argonParams struct {
	version int
	memory  uint32
	time    uint32
	threads uint8
	salt    []byte
	key     []byte
}

// HashPassword returns a PHC-formatted Argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func parsePHC(phc string) (argonParams, error) {
	var p argonParams
	parts := strings.Split(strings.TrimPrefix(phc, "$"), "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return p, errors.New("auth: malformed phc string")
	}
	if _, err := fmt.Sscanf(parts[1], "v=%d", &p.version); err != nil {
		return p, err
	}
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, err
	}
	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[3]); err != nil {
		return p, err
	}
	if p.key, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return p, err
	}
	return p, nil
}

// VerifyPassword checks a password against a PHC Argon2id hash.
func VerifyPassword(password, phc string) bool {
	p, err := parsePHC(phc)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), p.salt, p.time, p.memory, p.threads, uint32(len(p.key)))
	return subtle.ConstantTimeCompare(got, p.key) == 1
}

// ---- RS256 JWT sessions ----

type TokenType string

const (
	TokenAccess  TokenType = "access"
	TokenRefresh TokenType = "refresh"
)

type Claims struct {
	UserID    string    `json:"uid"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenIssuer signs/verifies RS256 session tokens.
// Refresh tokens carry a unique jti so the API can rotate/revoke them
// (audit finding #2); access tokens are short-lived stateless JWTs.
type TokenIssuer struct {
	private    *rsa.PrivateKey
	public     *rsa.PublicKey
	nowFunc    func() time.Time
	accessTTL  time.Duration
	refreshTTL time.Duration
	jtiFunc    func() string
}

func NewTokenIssuer(private *rsa.PrivateKey, accessTTL, refreshTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{
		private:    private,
		public:     &private.PublicKey,
		nowFunc:    time.Now,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (t *TokenIssuer) Issue(userID string, typ TokenType, jti string) (string, error) {
	now := t.nowFunc()
	ttl := t.accessTTL
	if typ == TokenRefresh {
		ttl = t.refreshTTL
		if jti == "" {
			return "", errors.New("auth: refresh tokens require a jti")
		}
	} else if typ != TokenAccess {
		return "", errors.New("auth: unknown token type")
	}
	claims := Claims{
		UserID:    userID,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "snowradar-api",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(t.private)
}

func (t *TokenIssuer) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		return t.public, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("snowradar-api"))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}
	return claims, nil
}
