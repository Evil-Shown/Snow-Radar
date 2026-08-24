// Package peer: device/peer lifecycle — allocate a tunnel address (ADR-004),
// persist the record, and render a client WireGuard config.
//
// PRIVACY INVARIANT: the CLIENT generates its own Curve25519 keypair on
// device; the API accepts only the PUBLIC key. The server never sees or
// stores private keys (SECURITY.md "local key generation").
package peer

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/evil-shown/snow-radar/api/internal/netalloc"
	"github.com/evil-shown/snow-radar/api/internal/store"
)

var (
	ErrNodeUnknown   = errors.New("peer: unknown node")
	ErrNotSubscribed = errors.New("peer: no active subscription")
	ErrTooManyPeers  = errors.New("peer: device limit reached")
	ErrBadPublicKey  = errors.New("peer: public key must be 32 bytes, base64-encoded")
)

const MaxPeersPerUser = 3

type Service struct {
	store         store.Store
	allocators    map[string]*entry
	serverPubKeys map[string]string          // nodeID -> wg/awg server public key (terraform output)
	endpoints     map[string]map[bool]string // nodeID -> stealth -> "host:port"
}

type entry struct {
	wg  *netalloc.Allocator
	awg *netalloc.Allocator
}

func NewService(st store.Store, serverPubKeys map[string]string, endpoints map[string]map[bool]string) (*Service, error) {
	s := &Service{store: st, allocators: map[string]*entry{}, serverPubKeys: serverPubKeys, endpoints: endpoints}
	for _, nodeID := range []string{"sgp", "fsn"} {
		if _, ok := serverPubKeys[nodeID]; !ok {
			continue
		}
		wg, err := netalloc.New(nodeID, false)
		if err != nil {
			return nil, err
		}
		awg, err := netalloc.New(nodeID, true)
		if err != nil {
			return nil, err
		}
		s.allocators[nodeID] = &entry{wg: wg, awg: awg}
	}
	return s, nil
}

// Connect provisions (or reuses) a peer for the user on the requested node.
// clientPubKey is the DEVICE-GENERATED WireGuard public key.
func (s *Service) Connect(userID, nodeID, clientPubKey string, stealth bool) (*store.Peer, string, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(clientPubKey)
	if err != nil || len(pubBytes) != 32 {
		return nil, "", ErrBadPublicKey
	}

	sub, err := s.store.GetSubscription(userID)
	if err != nil {
		return nil, "", ErrNotSubscribed
	}
	if sub.State != "active" && sub.State != "trialing" {
		return nil, "", ErrNotSubscribed
	}

	entryA, ok := s.allocators[nodeID]
	if !ok {
		return nil, "", ErrNodeUnknown
	}

	existing, _ := s.store.PeersByUser(userID)
	// Idempotent ONLY on exact (node, stealth, pubkey) match. Matching
	// without the pubkey let a second DEVICE inherit the first device's
	// tunnel identity — found by TestDeviceLimitEnforced.
	for _, p := range existing {
		if p.NodeID == nodeID && p.Stealth == stealth && p.PublicKey == clientPubKey {
			return p, s.renderClientConfig(p), nil
		}
	}
	if len(existing) >= MaxPeersPerUser {
		return nil, "", ErrTooManyPeers
	}

	alloc := entryA.wg
	if stealth {
		alloc = entryA.awg
	}
	addr, err := alloc.Allocate(userID)
	if err != nil {
		return nil, "", fmt.Errorf("peer: allocate: %w", err)
	}

	p := &store.Peer{
		ID:        newID(),
		UserID:    userID,
		NodeID:    nodeID,
		Stealth:   stealth,
		Address:   addr,
		PublicKey: clientPubKey,
		CreatedAt: time.Now().Unix(),
		Active:    true,
	}
	if err := s.store.SavePeer(p); err != nil {
		_ = alloc.Release(addr, userID)
		return nil, "", err
	}
	return p, s.renderClientConfig(p), nil
}

func (s *Service) renderClientConfig(p *store.Peer) string {
	ep := s.endpoints[p.NodeID][p.Stealth]
	cfg := fmt.Sprintf("[Interface]\nAddress = %s\nDNS = 1.1.1.1, 1.0.0.1\n\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = 0.0.0.0/0\nPersistentKeepalive = 25\n",
		p.Address, s.serverPubKeys[p.NodeID], ep)
	return cfg
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
