package netalloc

import (
	"errors"
	"testing"

	"net/netip"
)

func TestAllocateSequentialSkipsServerAddress(t *testing.T) {
	a, err := New("sgp", false)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := a.Allocate("p1")
	want := netip.MustParsePrefix("10.20.0.2/24")
	if first != want {
		t.Fatalf("first lease = %v, want %v (server .1 must be skipped)", first, want)
	}
	second, _ := a.Allocate("p2")
	if second.Addr() != netip.MustParseAddr("10.20.0.3") {
		t.Fatalf("second lease = %v, want 10.20.0.3", second)
	}
}

func TestStealthUsesSeparateBlock(t *testing.T) {
	a, err := New("sgp", true)
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Allocate("p1")
	if lease.Addr().String() != "10.20.1.2" {
		t.Fatalf("stealth lease = %v, want 10.20.1.2", lease)
	}
}

func TestUnknownNode(t *testing.T) {
	if _, err := New("nyc", false); !errors.Is(err, ErrUnknownNode) {
		t.Fatalf("want ErrUnknownNode, got %v", err)
	}
}

func TestExhaustionWrapsAndEventuallyFails(t *testing.T) {
	a, _ := New("fsn", false)
	for i := 0; i < 253; i++ {
		if _, err := a.Allocate("peer"); err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	if _, err := a.Allocate("one-too-many"); !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("want ErrPoolExhausted, got %v", err)
	}
}

func TestReleaseOwnershipEnforced(t *testing.T) {
	a, _ := New("sgp", false)
	lease, _ := a.Allocate("owner")
	if err := a.Release(lease, "attacker"); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("wrong-owner release should fail, got %v", err)
	}
	if got := a.HeldBy(lease); got != "owner" {
		t.Fatalf("lease stolen? held by %q", got)
	}
	if err := a.Release(lease, "owner"); err != nil {
		t.Fatalf("legit release failed: %v", err)
	}
	reused, _ := a.Allocate("next")
	if reused != lease {
		t.Fatalf("freed address should be reusable, got %v want %v", reused, lease)
	}
}

func TestConcurrentAllocationNoDuplicates(t *testing.T) {
	a, _ := New("sgp", true)
	const n = 100
	results := make(chan netip.Prefix, n)
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			p, err := a.Allocate("x")
			if err != nil {
				t.Errorf("alloc: %v", err)
			}
			results <- p
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	seen := map[netip.Prefix]bool{}
	close(results)
	for p := range results {
		if seen[p] {
			t.Fatalf("duplicate lease handed out: %v", p)
		}
		seen[p] = true
	}
	if a.InUse() != n {
		t.Fatalf("InUse=%d want %d", a.InUse(), n)
	}
}
