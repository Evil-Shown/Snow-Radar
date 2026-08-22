package peer

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/evil-shown/snow-radar/api/internal/store"
)

func testPubKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	st := store.NewMemory()
	_ = st.UpsertSubscription(&store.Subscription{UserID: "", Provider: "paddle", External: "x", State: "active"})
	s, err := NewService(st,
		map[string]string{"sgp": "SRVKEY=", "fsn": "SRVKEY2="},
		map[string]map[bool]string{
			"sgp": {false: "sgp.snowradar.app:51820", true: "sgp.snowradar.app:51821"},
			"fsn": {false: "fsn.snowradar.app:51820", true: "fsn.snowradar.app:51821"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func seedUser(t *testing.T, s *Service, id, state string) {
	t.Helper()
	if err := s.store.UpsertSubscription(&store.Subscription{UserID: id, Provider: "paddle", External: "sub_" + id, State: state}); err != nil {
		t.Fatal(err)
	}
}

func TestConnectAllocatesFromCorrectBlock(t *testing.T) {
	s := newTestService(t)
	seedUser(t, s, "u1", "active")

	p, cfg, err := s.Connect("u1", "sgp", testPubKey(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Address.Addr().String() != "10.20.0.2" {
		t.Fatalf("sgp wg lease = %v, want 10.20.0.2", p.Address)
	}
	if !strings.Contains(cfg, "Endpoint = sgp.snowradar.app:51820") {
		t.Fatalf("config endpoint wrong:\n%s", cfg)
	}

	p2, _, err := s.Connect("u1", "sgp", testPubKey(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Address.Addr().String() != "10.20.1.2" {
		t.Fatalf("sgp awg lease = %v, want 10.20.1.2 (stealth block)", p2.Address)
	}
}

func TestConnectRequiresActiveSubscription(t *testing.T) {
	s := newTestService(t)
	if _, _, err := s.Connect("ghost", "sgp", testPubKey(t), false); err != ErrNotSubscribed {
		t.Fatalf("want ErrNotSubscribed, got %v", err)
	}
	seedUser(t, s, "u2", "cancelled")
	if _, _, err := s.Connect("u2", "sgp", testPubKey(t), false); err != ErrNotSubscribed {
		t.Fatalf("cancelled sub should not connect, got %v", err)
	}
}

func TestConnectRejectsBadPublicKey(t *testing.T) {
	s := newTestService(t)
	seedUser(t, s, "u3", "active")
	if _, _, err := s.Connect("u3", "sgp", "not-base64!!", false); err != ErrBadPublicKey {
		t.Fatalf("want ErrBadPublicKey, got %v", err)
	}
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	if _, _, err := s.Connect("u3", "sgp", short, false); err != ErrBadPublicKey {
		t.Fatalf("short key accepted")
	}
}

func TestDeviceLimitEnforced(t *testing.T) {
	s := newTestService(t)
	seedUser(t, s, "u4", "active")
	for i := 0; i < MaxPeersPerUser; i++ {
		if _, _, err := s.Connect("u4", "sgp", testPubKey(t), false); err != nil {
			t.Fatalf("peer %d: %v", i, err)
		}
	}
	if _, _, err := s.Connect("u4", "fsn", testPubKey(t), false); err != ErrTooManyPeers {
		t.Fatalf("want ErrTooManyPeers, got %v", err)
	}
}

func TestIdempotentConnectReusesPeer(t *testing.T) {
	s := newTestService(t)
	seedUser(t, s, "u5", "active")
	key := testPubKey(t)
	p1, _, _ := s.Connect("u5", "fsn", key, false)
	p2, _, _ := s.Connect("u5", "fsn", key, false)
	if p1.ID != p2.ID {
		t.Fatal("same node+stealth+user should reuse the peer")
	}
}
