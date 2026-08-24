// Package store: persistence contracts. Postgres-backed implementation lands
// with migrations; the memory store keeps tests hermetic.
package store

import (
	"errors"

	"net/netip"
)

var (
	ErrNotFound      = errors.New("store: not found")
	ErrTokenReplayed = errors.New("store: refresh token already consumed")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

type Peer struct {
	ID       string
	UserID   string
	NodeID   string
	Stealth  bool
	Address  netip.Prefix
	PublicKey string
	CreatedAt int64
	Active   bool
}

type Subscription struct {
	UserID   string
	Provider string
	External string
	State    string
}

type Store interface {
	CreateUser(u *User) error
	GetUserByEmail(email string) (*User, error)
	GetUser(id string) (*User, error)

	SavePeer(p *Peer) error
	PeersByUser(userID string) ([]*Peer, error)
	RevokePeer(peerID, userID string) error

	UpsertSubscription(s *Subscription) error
	GetSubscription(userID string) (*Subscription, error)

	// Refresh-token rotation state (jti-tracked). Consume is atomic and
	// single-use; a replayed jti returns ErrTokenReplayed WITH its userID
	// so callers can burn the whole family.
	SaveRefreshToken(jti, userID string) error
	ConsumeRefreshToken(jti string) (userID string, err error)
	RevokeAllRefreshTokens(userID string) error
}
