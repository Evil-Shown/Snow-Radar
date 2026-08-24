package billing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrBadCheckoutToken   = errors.New("billing: invalid checkout token")
	ErrExpiredCheckoutTok = errors.New("billing: checkout token expired")
)

// CheckoutService mints short-lived, HMAC-bound checkout sessions.
//
// SECURITY MODEL (closes residual risk B-webhook-takeover): the control
// plane issues a one-time-ish signed token when an authenticated user starts
// a payment. The provider echoes it back verbatim in custom fields. The
// webhook therefore NEVER trusts a provider-controlled user identifier —
// identity comes from OUR signature over OUR record of who was checking out.
type CheckoutService struct {
	secret []byte
	now    func() time.Time
}

func NewCheckoutService(secret []byte) *CheckoutService {
	return &CheckoutService{secret: secret, now: time.Now}
}

type checkoutClaims struct {
	UserID string `json:"u"`
	Expiry int64  `json:"e"`
	Nonce  string `json:"n"`
}

// Mint returns "<b64url(claims)>.<b64url(hmac)>".
func (c *CheckoutService) Mint(userID string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", ErrBadCheckoutToken
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("billing: nonce: %w", err)
	}
	payload, err := json.Marshal(checkoutClaims{
		UserID: userID,
		Expiry: c.now().Add(ttl).Unix(),
		Nonce:  base64.RawURLEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + c.sign(body), nil
}

func (c *CheckoutService) sign(body string) string {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify returns the bound userID or an error. Signature checked in
// constant time; expiry enforced against our clock, not the provider's.
func (c *CheckoutService) Verify(token string) (string, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" {
		return "", ErrBadCheckoutToken
	}
	expected := c.sign(body)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", ErrBadCheckoutToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", ErrBadCheckoutToken
	}
	var claims checkoutClaims
	if err := json.Unmarshal(raw, &claims); err != nil || claims.UserID == "" {
		return "", ErrBadCheckoutToken
	}
	if c.now().After(time.Unix(claims.Expiry, 0)) {
		return "", ErrExpiredCheckoutTok
	}
	return claims.UserID, nil
}
