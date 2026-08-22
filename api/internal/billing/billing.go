// Package billing: subscription state synced from payment providers.
// Signature verification implemented per provider spec; state transitions
// are the only mutation path (webhooks are untrusted input otherwise).
package billing

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrBadSignature = errors.New("billing: webhook signature verification failed")
	ErrBadPayload   = errors.New("billing: malformed webhook payload")
)

// Paddle: `Paddle-Signature: ts=<unix>;h1=<hex hmac>` where hmac is
// HMAC-SHA256(secret, ts + ":" + rawBody).
func VerifyPaddle(secret string, header string, rawBody []byte) error {
	var ts, h1 string
	for _, kv := range splitSemi(header) {
		if len(kv) > 3 && kv[:3] == "ts=" {
			ts = kv[3:]
		}
		if len(kv) > 3 && kv[:3] == "h1=" {
			h1 = kv[3:]
		}
	}
	if ts == "" || h1 == "" {
		return ErrBadSignature
	}
	// Replay guard: reject timestamps older than 5 minutes.
	var tsUnix int64
	if _, err := fmt.Sscanf(ts, "%d", &tsUnix); err != nil || time.Since(time.Unix(tsUnix, 0)) > 5*time.Minute {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + ":"))
	mac.Write(rawBody)
	if !hmac.Equal([]byte(h1), []byte(hex.EncodeToString(mac.Sum(nil)))) {
		return ErrBadSignature
	}
	return nil
}

func splitSemi(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// PayHere: MD5 upper hash of concatenated merchant params sent as `hash`.
// MD5 here is the provider's protocol, not our choice; it is verified
// server-side against our merchant secret and is not used for secrecy.
func VerifyPayHere(merchantSecret string, merchantID, orderID, amount, currency, statusCode, providedHash string) bool {
	md5hex := func(s string) string {
		sum := md5.Sum([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	md5upper := func(s string) string { return strings.ToUpper(md5hex(s)) }
	expect := md5upper(merchantID + orderID + amount + currency + md5upper(merchantSecret) + statusCode)
	return hmac.Equal([]byte(expect), []byte(strings.ToUpper(providedHash)))
}

// SubscriptionState is the minimal state machine the API needs.
type SubscriptionState string

const (
	StateTrialing  SubscriptionState = "trialing"
	StateActive    SubscriptionState = "active"
	StatePastDue   SubscriptionState = "past_due"
	StateCancelled SubscriptionState = "cancelled"
)

// Event is a normalized provider-agnostic subscription event.
type Event struct {
	Provider string            `json:"provider"`
	External string            `json:"external_id"`
	UserID   string            `json:"user_id"`
	State    SubscriptionState `json:"state"`
}

// ParsePaddle extracts a normalized Event from a webhook body.
func ParsePaddle(raw []byte) (*Event, error) {
	var payload struct {
		EventType string `json:"event_type"`
		Data      struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			CustomData *struct {
				UserID string `json:"user_id"`
			} `json:"custom_data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadPayload, err)
	}
	if payload.Data.ID == "" || payload.Data.CustomData == nil {
		return nil, ErrBadPayload
	}
	state := map[string]SubscriptionState{
		"active":       StateActive,
		"trialing":     StateTrialing,
		"past_due":     StatePastDue,
		"canceled":     StateCancelled,
		"cancelled":    StateCancelled,
		"subscription_updated": StateActive,
	}[payload.Data.Status]
	if state == "" && payload.Data.Status != "" {
		state = SubscriptionState(payload.Data.Status)
	}
	return &Event{Provider: "paddle", External: payload.Data.ID, UserID: payload.Data.CustomData.UserID, State: state}, nil
}

// ReadBody caps webhook body size (DoS guard) before any parsing.
func ReadBody(r *http.Request, maxBytes int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, maxBytes))
}
