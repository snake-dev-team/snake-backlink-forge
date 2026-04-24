// Package bot — Redis-backed FSM state store.
// Each Telegram user has at most one active state key; absence of the key means "idle".
package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// State represents the current FSM position for a single Telegram user.
// Data carries per-state payload (e.g. package_code during buy flow).
// ExpiresAt is stored for informational purposes; Redis TTL is authoritative.
type State struct {
	Name      string          `json:"state"`
	Data      json.RawMessage `json:"data"`
	ExpiresAt int64           `json:"expires_at"`
}

// Store is the Redis-backed FSM state repository.
type Store struct {
	rdb *goredis.Client
}

// NewStore returns a Store backed by the provided Redis client.
func NewStore(rdb *goredis.Client) *Store {
	return &Store{rdb: rdb}
}

// stateKey returns the canonical Redis key for a user's FSM state.
func stateKey(tgID int64) string {
	return fmt.Sprintf("tg:state:%d", tgID)
}

// Load retrieves the FSM state for tgID.
// Returns goredis.Nil (wrapped) when no state exists (user is effectively idle).
func (s *Store) Load(ctx context.Context, tgID int64) (State, error) {
	raw, err := s.rdb.Get(ctx, stateKey(tgID)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return State{}, fmt.Errorf("bot/state: no state for %d: %w", tgID, goredis.Nil)
		}
		return State{}, fmt.Errorf("bot/state: redis get: %w", err)
	}

	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return State{}, fmt.Errorf("bot/state: unmarshal: %w", err)
	}

	return st, nil
}

// Save persists st for tgID with the given TTL.
// A zero TTL is rejected to prevent accidental permanent keys.
func (s *Store) Save(ctx context.Context, tgID int64, st State, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("bot/state: ttl must be positive, got %v", ttl)
	}

	st.ExpiresAt = time.Now().Add(ttl).Unix()

	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("bot/state: marshal: %w", err)
	}

	if err := s.rdb.Set(ctx, stateKey(tgID), raw, ttl).Err(); err != nil {
		return fmt.Errorf("bot/state: redis set: %w", err)
	}

	return nil
}

// Clear removes the FSM state for tgID (returns the user to idle).
// A missing key is not an error.
func (s *Store) Clear(ctx context.Context, tgID int64) error {
	if err := s.rdb.Del(ctx, stateKey(tgID)).Err(); err != nil {
		return fmt.Errorf("bot/state: redis del: %w", err)
	}
	return nil
}
