package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

// Memory is a concurrency-safe in-memory Store for tests/dev.
type Memory struct {
	mu      sync.RWMutex
	users   map[string]*User // by id
	byEmail map[string]*User
	peers   map[string]*Peer // by id
	subs    map[string]*Subscription
	refresh map[string]*refreshRow
}

func NewMemory() *Memory {
	return &Memory{
		users:   map[string]*User{},
		byEmail: map[string]*User{},
		peers:   map[string]*Peer{},
		subs:    map[string]*Subscription{},
		refresh: map[string]*refreshRow{},
	}
}

func (m *Memory) CreateUser(u *User) error {
	m.mu.Lock(); defer m.mu.Unlock()
	if _, ok := m.byEmail[u.Email]; ok {
		return errors.New("email already registered")
	}
	if u.ID == "" {
		// AUDIT FINDING #7: sequential "u-N" IDs were enumerable, which
		// combined with webhook user binding made targeted attacks trivial.
		// Use 128-bit random IDs everywhere.
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		u.ID = hex.EncodeToString(b)
	}
	cp := *u
	m.users[u.ID] = &cp
	m.byEmail[u.Email] = &cp
	return nil
}

func (m *Memory) GetUserByEmail(email string) (*User, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	u, ok := m.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) GetUser(id string) (*User, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) SavePeer(p *Peer) error {
	m.mu.Lock(); defer m.mu.Unlock()
	cp := *p
	m.peers[p.ID] = &cp
	return nil
}

func (m *Memory) PeersByUser(userID string) ([]*Peer, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	var out []*Peer
	for _, p := range m.peers {
		if p.UserID == userID && p.Active {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *Memory) RevokePeer(peerID, userID string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	p, ok := m.peers[peerID]
	if !ok || p.UserID != userID {
		return ErrNotFound
	}
	p.Active = false
	return nil
}

func (m *Memory) UpsertSubscription(s *Subscription) error {
	m.mu.Lock(); defer m.mu.Unlock()
	cp := *s
	m.subs[s.UserID] = &cp
	return nil
}

func (m *Memory) GetSubscription(userID string) (*Subscription, error) {
	m.mu.RLock(); defer m.mu.RUnlock()
	s, ok := m.subs[userID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}

type refreshRow struct {
	userID   string
	consumed bool
}

func (m *Memory) SaveRefreshToken(jti, userID string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	m.refresh[jti] = &refreshRow{userID: userID}
	return nil
}

func (m *Memory) ConsumeRefreshToken(jti string) (string, error) {
	m.mu.Lock(); defer m.mu.Unlock()
	row, ok := m.refresh[jti]
	if !ok {
		return "", ErrNotFound
	}
	if row.consumed {
		// Replay: burn every outstanding token for this user.
		for _, other := range m.refresh {
			if other.userID == row.userID {
				other.consumed = true
			}
		}
		return row.userID, ErrTokenReplayed
	}
	row.consumed = true
	return row.userID, nil
}

func (m *Memory) RevokeAllRefreshTokens(userID string) error {
	m.mu.Lock(); defer m.mu.Unlock()
	for _, row := range m.refresh {
		if row.userID == userID {
			row.consumed = true
		}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
