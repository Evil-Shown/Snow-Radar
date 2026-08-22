package store

import (
	"sync"
)

// Memory is a concurrency-safe in-memory Store for tests/dev.
type Memory struct {
	mu    sync.RWMutex
	users map[string]*User // by id
	byEmail map[string]*User
	peers map[string]*Peer // by id
	subs  map[string]*Subscription
	seq   int
}

func NewMemory() *Memory {
	return &Memory{
		users:   map[string]*User{},
		byEmail: map[string]*User{},
		peers:   map[string]*Peer{},
		subs:    map[string]*Subscription{},
	}
}

func (m *Memory) CreateUser(u *User) error {
	m.mu.Lock(); defer m.mu.Unlock()
	if _, ok := m.byEmail[u.Email]; ok {
		return errors.New("email already registered")
	}
	m.seq++
	if u.ID == "" {
		u.ID = "u-" + itoa(m.seq)
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
