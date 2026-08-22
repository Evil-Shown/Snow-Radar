// Package store: persistence contracts. Postgres-backed implementation lands
// with migrations; the memory store keeps tests hermetic.
package store

import (
	"errors"

	"net/netip"
)

var ErrNotFound = errors.New("store: not found")

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
}
