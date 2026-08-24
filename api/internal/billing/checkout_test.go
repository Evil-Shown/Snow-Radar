package billing

import (
	"strings"
	"testing"
	"time"
)

func TestCheckoutMintVerifyRoundTrip(t *testing.T) {
	c := NewCheckoutService([]byte("test-secret-32-bytes-aaaaaaaaaaaa"))
	tok, err := c.Mint("user-abc", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Verify(tok)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if got != "user-abc" {
		t.Fatalf("uid = %q", got)
	}
}

func TestCheckoutTamperRejected(t *testing.T) {
	c := NewCheckoutService([]byte("secret"))
	tok, _ := c.Mint("user-a", time.Minute)

	parts := strings.Split(tok, ".")
	forged, _ := c.Mint("user-victim", time.Minute)
	victimBody := strings.Split(forged, ".")[0]

	// Swap the body but keep the original signature.
	if _, err := c.Verify(victimBody + "." + parts[1]); err == nil {
		t.Fatal("swapped body accepted")
	}
	// Truncated signature.
	if _, err := c.Verify(tok[:len(tok)-2]); err == nil {
		t.Fatal("corrupted token accepted")
	}
}

func TestCheckoutWrongSecretRejected(t *testing.T) {
	a := NewCheckoutService([]byte("secret-one"))
	b := NewCheckoutService([]byte("secret-two"))
	tok, _ := a.Mint("u", time.Minute)
	if _, err := b.Verify(tok); err == nil {
		t.Fatal("token from foreign secret accepted")
	}
}

func TestCheckoutExpiryEnforced(t *testing.T) {
	c := NewCheckoutService([]byte("secret"))
	c.now = func() time.Time { return time.Unix(1000, 0) }
	tok, _ := c.Mint("u", time.Minute)

	c.now = func() time.Time { return time.Unix(1061, 0) } // past expiry
	if _, err := c.Verify(tok); err != ErrExpiredCheckoutTok {
		t.Fatalf("want ErrExpiredCheckoutTok, got %v", err)
	}
}

func TestCheckoutGarbage(t *testing.T) {
	c := NewCheckoutService([]byte("s"))
	for _, bad := range []string{"", ".", "abc.def", "...."} {
		if _, err := c.Verify(bad); err == nil {
			t.Fatalf("garbage accepted: %q", bad)
		}
	}
}
