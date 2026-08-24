package api

import (
	"testing"
)

func TestRefreshRotationSingleUse(t *testing.T) {
	rs := newRefreshStore()
	rs.issue("jti-1", "user-a")
	rs.issue("jti-2", "user-a") // sibling issued BEFORE the replay

	rec, ok := rs.consume("jti-1")
	if !ok || rec.userID != "user-a" {
		t.Fatal("first use must succeed")
	}

	// Replay: must fail...
	if _, ok := rs.consume("jti-1"); ok {
		t.Fatal("replayed refresh token accepted")
	}
	// ...and burn outstanding family members (issued before detection).
	if _, ok := rs.consume("jti-2"); ok {
		t.Fatal("sibling token survived replay detection (family not revoked)")
	}
	// A brand-new login AFTER detection gets a fresh, valid session.
	rs.issue("jti-3", "user-a")
	if _, ok := rs.consume("jti-3"); !ok {
		t.Fatal("post-incident re-login wrongly revoked")
	}
	// Other users unaffected.
	rs.issue("jti-4", "user-b")
	if _, ok := rs.consume("jti-4"); !ok {
		t.Fatal("innocent user's token revoked")
	}
}

func TestRefreshConsumeUnknownJTI(t *testing.T) {
	rs := newRefreshStore()
	if _, ok := rs.consume("does-not-exist"); ok {
		t.Fatal("unknown jti accepted")
	}
}
