package api

import (
	"errors"
	"testing"

	"github.com/evil-shown/snow-radar/api/internal/store"
)

func TestRefreshRotationSingleUse(t *testing.T) {
	sm := newSessionManager(store.NewMemory())
	sm.issue("jti-1", "user-a")
	sm.issue("jti-2", "user-a") // sibling issued BEFORE the replay

	uid, err := sm.consume("jti-1")
	if err != nil || uid != "user-a" {
		t.Fatal("first use must succeed")
	}

	// Replay: must fail...
	if _, err := sm.consume("jti-1"); !errors.Is(err, store.ErrTokenReplayed) {
		t.Fatalf("replayed refresh token accepted (err=%v)", err)
	}
	// ...and burn outstanding family members (issued before detection).
	if _, err := sm.consume("jti-2"); err == nil {
		t.Fatal("sibling token survived replay detection (family not revoked)")
	}
	// A brand-new login AFTER detection gets a fresh, valid session.
	sm.issue("jti-3", "user-a")
	if _, err := sm.consume("jti-3"); err != nil {
		t.Fatalf("post-incident re-login wrongly revoked: %v", err)
	}
	// Other users unaffected.
	sm.issue("jti-4", "user-b")
	if _, err := sm.consume("jti-4"); err != nil {
		t.Fatalf("innocent user's token revoked: %v", err)
	}
}

func TestRefreshConsumeUnknownJTI(t *testing.T) {
	sm := newSessionManager(store.NewMemory())
	if _, err := sm.consume("does-not-exist"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown jti accepted: %v", err)
	}
}
