//go:build integration

// Integration tests against real Postgres. Skipped unless explicitly run with:
//
//	docker compose -f dev/docker-compose.yml up -d
//	TEST_DATABASE_URL='postgres://snowradar:devonly-not-a-secret@127.0.0.1:54329/snowradar' \
//	  go test -tags integration ./internal/store/ -v
package store

import (
	"context"
	"errors"
	"os"
	"net/netip"
	"testing"
	"time"
)

func testDB(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pg, err := NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pg.Close)

	// Clean slate per test run (dev DB only).
	for _, stmt := range []string{
		`DELETE FROM refresh_tokens`, `DELETE FROM peers`,
		`DELETE FROM subscriptions`, `DELETE FROM users`,
	} {
		if _, err := pg.pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	return pg
}

func TestPostgresUserAndPeerLifecycle(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	u := &User{Email: "it@example.com", PasswordHash: "hash"}
	if err := pg.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID == "" || len(u.ID) < 16 {
		t.Fatalf("expected random id, got %q", u.ID)
	}
	got, err := pg.GetUserByEmail("it@example.com")
	if err != nil || got.ID != u.ID {
		t.Fatalf("lookup by email failed: %v", err)
	}

	addr := netip.MustParsePrefix("10.20.0.2/24")
	if err := pg.SavePeer(&Peer{ID: "p1", UserID: u.ID, NodeID: "sgp", Address: addr, PublicKey: "KEY1", CreatedAt: 1, Active: true}); err != nil {
		t.Fatalf("save peer: %v", err)
	}
	peers, _ := pg.PeersByUser(u.ID)
	if len(peers) != 1 || !peers[0].Address.Addr().Compare(addr.Addr()).IsZero() {
		t.Fatalf("peer round-trip wrong: %+v", peers)
	}

	// Duplicate address must be rejected by UNIQUE constraint.
	err = pg.SavePeer(&Peer{ID: "p2", UserID: u.ID, NodeID: "sgp", Address: addr, PublicKey: "KEY2", CreatedAt: 2, Active: true})
	if err == nil {
		t.Fatal("duplicate tunnel address accepted (lease backstop missing)")
	}

	if err := pg.RevokePeer("p1", u.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := pg.RevokePeer("p1", u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double revoke should be NotFound, got %v", err)
	}
	_ = ctx
}

func TestPostgresRefreshTokenAtomicConsume(t *testing.T) {
	pg := testDB(t)

	u := &User{Email: "rt@example.com", PasswordHash: "h"}
	if err := pg.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := pg.SaveRefreshToken("jti-x", u.ID); err != nil {
		t.Fatal(err)
	}
	uid, err := pg.ConsumeRefreshToken("jti-x")
	if err != nil || uid != u.ID {
		t.Fatalf("first consume failed: %v", err)
	}
	// Replay: returns userID + ErrTokenReplayed.
	uid, err = pg.ConsumeRefreshToken("jti-x")
	if !errors.Is(err, ErrTokenReplayed) || uid != u.ID {
		t.Fatalf("replay not detected: uid=%q err=%v", uid, err)
	}
	if err := pg.RevokeAllRefreshTokens(u.ID); err != nil {
		t.Fatal(err)
	}
}
