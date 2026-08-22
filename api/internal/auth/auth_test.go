package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashVerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword("wrong password", hash) {
		t.Fatal("invalid password accepted")
	}
}

func TestVerifyRejectsGarbagePHC(t *testing.T) {
	for _, bad := range []string{"", "$argon2id$v=19$m=1,t=1,p=1$", "bcrypt$abc", "$argon2id$v=19$m=x,t=y,p=z$a$b"} {
		if VerifyPassword("pw", bad) {
			t.Fatalf("garbage phc accepted: %q", bad)
		}
	}
}

func TestUniqueSaltsPerHash(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("two hashes of same password must differ (salt)")
	}
}

func TestJWTIssueVerify(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	issuer := NewTokenIssuer(key, 15*time.Minute, 30*24*time.Hour)

	access, err := issuer.Issue("user-123", TokenAccess, "")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := issuer.Verify(access)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Fatalf("uid = %q", claims.UserID)
	}
	if claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 15*time.Minute {
		t.Fatalf("access TTL wrong: %v", claims.ExpiresAt.Sub(claims.IssuedAt.Time))
	}
}

func TestJWTTamperRejected(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	issuer := NewTokenIssuer(key, time.Minute, time.Hour)

	token, _ := issuer.Issue("victim", TokenAccess, "")

	// signed by a different key
	forged, _ := NewTokenIssuer(other, time.Minute, time.Hour).Issue("attacker", TokenAccess, "")
	if forged == token {
		t.Fatal("keys produced identical tokens")
	}
	if _, err := issuer.Verify(forged); err == nil {
		t.Fatal("token signed by foreign key accepted")
	}
	if _, err := issuer.Verify(token + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
}

func TestJWTExpiredRejected(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	issuer := NewTokenIssuer(key, -time.Second, time.Hour) // already expired
	token, _ := issuer.Issue("u", TokenAccess, "")
	if _, err := issuer.Verify(token); err == nil {
		t.Fatal("expired token accepted")
	}
}
