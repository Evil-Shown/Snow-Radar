// Package netalloc allocates tunnel IPs per ADR-004.
//
// Node blocks (single source: infra/configs/subnets.env):
//
//	sgp: wg 10.20.0.0/24 (.1 = server), awg 10.20.1.0/24
//	fsn: wg 10.21.0.0/24,                awg 10.21.1.0/24
//
// Client leases come from .2–.254 of the node's block for the chosen protocol.
package netalloc

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

var (
	ErrPoolExhausted   = errors.New("netalloc: address pool exhausted")
	ErrUnknownNode     = errors.New("netalloc: unknown node id")
	ErrAlreadyHeld     = errors.New("netalloc: address already held")
	ErrNotHeld         = errors.New("netalloc: address not held by peer")
)

// Node describes one exit node's client pools.
type Node struct {
	ID       string
	WGPrefix netip.Prefix  // standard WireGuard pool
	AWGPrefix netip.Prefix // stealth AmneziaWG pool
}

// DefaultNodes mirrors infra/configs/subnets.env exactly. If subnets.env
// changes, change this table in the same commit.
var DefaultNodes = []Node{
	{ID: "sgp", WGPrefix: mustPrefix("10.20.0.0/24"), AWGPrefix: mustPrefix("10.20.1.0/24")},
	{ID: "fsn", WGPrefix: mustPrefix("10.21.0.0/24"), AWGPrefix: mustPrefix("10.21.1.0/24")},
}

func mustPrefix(s string) netip.Prefix {
	p := netip.MustParsePrefix(s)
	return p
}

// Allocator hands out single-host leases from a /24 pool, skipping the server
// address (.1). Released addresses are reused immediately (free-list).
// Safe for concurrent use.
type Allocator struct {
	mu      sync.Mutex
	node    Node
	stealth bool // false => WGPrefix, true => AWGPrefix
	held    map[netip.Addr]string
	free    []netip.Addr // released addresses, reused first
	next    int          // next host octet to try when free-list is empty
}

func New(nodeID string, stealth bool) (*Allocator, error) {
	for _, n := range DefaultNodes {
		if n.ID == nodeID {
			return &Allocator{node: n, stealth: stealth, held: map[netip.Addr]string{}, next: 2}, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownNode, nodeID)
}

func (a *Allocator) prefix() netip.Prefix {
	if a.stealth {
		return a.node.AWGPrefix
	}
	return a.node.WGPrefix
}

// Allocate returns the next free lease for peerID, reusing released
// addresses before extending into fresh territory.
func (a *Allocator) Allocate(peerID string) (netip.Prefix, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Fast path: hand back a released address.
	if n := len(a.free); n > 0 {
		addr := a.free[n-1]
		a.free = a.free[:n-1]
		a.held[addr] = peerID
		return netip.PrefixFrom(addr, a.prefix().Bits()), nil
	}

	base := a.prefix()
	for i := 0; i < 253; i++ {
		octet := a.next
		a.next++
		if a.next > 254 {
			a.next = 2
		}
		addr := netip.AddrFrom4([4]byte{
			base.Addr().As4()[0],
			base.Addr().As4()[1],
			base.Addr().As4()[2],
			byte(octet),
		})
		if _, taken := a.held[addr]; !taken {
			a.held[addr] = peerID
			return netip.PrefixFrom(addr, base.Bits()), nil
		}
	}
	return netip.Prefix{}, ErrPoolExhausted
}

// Release frees the lease if (and only if) peerID owns it.
func (a *Allocator) Release(addr netip.Prefix, peerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	owner, ok := a.held[addr.Addr()]
	if !ok || owner != peerID {
		return ErrNotHeld
	}
	delete(a.held, addr.Addr())
	a.free = append(a.free, addr.Addr())
	return nil
}

// HeldBy reports which peer holds an address ("" when free).
func (a *Allocator) HeldBy(addr netip.Prefix) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.held[addr.Addr()]
}

// InUse reports current allocation count.
func (a *Allocator) InUse() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.held)
}
