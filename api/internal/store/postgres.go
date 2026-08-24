package store

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a pgx-backed Store. All access is via parameterized queries
// (SECURITY.md: "no raw SQL string interpolation").
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: dsn: %w", err)
	}
	// Sensible bounds for a single control-plane instance.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

var _ Store = (*Postgres)(nil)

func (p *Postgres) CreateUser(u *User) error {
	return p.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		u.Email, u.PasswordHash,
	).Scan(&u.ID)
}

const userCols = `id, email, password_hash`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (p *Postgres) GetUserByEmail(email string) (*User, error) {
	return scanUser(p.pool.QueryRow(context.Background(),
		`SELECT `+userCols+` FROM users WHERE email = $1`, email))
}

func (p *Postgres) GetUser(id string) (*User, error) {
	return scanUser(p.pool.QueryRow(context.Background(),
		`SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (p *Postgres) SavePeer(peer *Peer) error {
	_, err := p.pool.Exec(context.Background(),
		`INSERT INTO peers (id, user_id, node_id, stealth, address, public_key, created_at, active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (id) DO UPDATE SET active = EXCLUDED.active`,
		peer.ID, peer.UserID, peer.NodeID, peer.Stealth, peer.Address.String(),
		peer.PublicKey, peer.CreatedAt, peer.Active,
	)
	return err
}

func (p *Postgres) PeersByUser(userID string) ([]*Peer, error) {
	rows, err := p.pool.Query(context.Background(),
		`SELECT id, user_id, node_id, stealth, address, public_key, created_at, active
		 FROM peers WHERE user_id = $1 AND active`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Peer
	for rows.Next() {
		var peer Peer
		var addr string
		if err := rows.Scan(&peer.ID, &peer.UserID, &peer.NodeID, &peer.Stealth,
			&addr, &peer.PublicKey, &peer.CreatedAt, &peer.Active); err != nil {
			return nil, err
		}
		prefix, err := netip.ParsePrefix(addr)
		if err != nil {
			return nil, err
		}
		peer.Address = prefix
		out = append(out, &peer)
	}
	return out, rows.Err()
}

func (p *Postgres) RevokePeer(peerID, userID string) error {
	tag, err := p.pool.Exec(context.Background(),
		`UPDATE peers SET active = FALSE WHERE id = $1 AND user_id = $2 AND active`,
		peerID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UpsertSubscription(s *Subscription) error {
	_, err := p.pool.Exec(context.Background(),
		`INSERT INTO subscriptions (user_id, provider, external_id, state, updated_at)
		 VALUES ($1,$2,$3,$4, now())
		 ON CONFLICT (user_id, provider)
		 DO UPDATE SET external_id = EXCLUDED.external_id,
		              state = EXCLUDED.state,
		              updated_at = now()`,
		s.UserID, s.Provider, s.External, s.State,
	)
	return err
}

func (p *Postgres) GetSubscription(userID string) (*Subscription, error) {
	var s Subscription
	err := p.pool.QueryRow(context.Background(),
		`SELECT user_id, provider, external_id, state
		 FROM subscriptions
		 WHERE user_id = $1
		 ORDER BY (state = 'active') DESC, updated_at DESC
		 LIMIT 1`, userID).
		Scan(&s.UserID, &s.Provider, &s.External, &s.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

