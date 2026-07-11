package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Lock is the play-lock payload stored at saves/<worldGUID>/lock.json.
// Advisory only (object storage has no CAS); TTL makes stale locks ignorable.
type Lock struct {
	Player      string `json:"player"`
	AcquiredAt  int64  `json:"acquired_at"` // unix millis
}

// LockKey returns the cloud key for a world's play lock.
func LockKey(worldGUID string) string {
	return "saves/" + worldGUID + "/lock.json"
}

// LockManager provides advisory play-lock operations over a Storage.
type LockManager struct {
	Store Storage
	TTL   time.Duration
}

// LockStatus describes the current lock for a world.
type LockStatus struct {
	Held      bool
	Stale     bool
	Lock      Lock
}

// Status reads the lock. Held=true means a fresh lock exists; Stale=true means
// a lock exists but is older than the TTL (overridable).
func (m *LockManager) Status(ctx context.Context, worldGUID string) (LockStatus, error) {
	data, err := m.Store.Get(ctx, LockKey(worldGUID))
	if err == ErrNotFound {
		return LockStatus{}, nil
	}
	if err != nil {
		return LockStatus{}, err
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return LockStatus{}, fmt.Errorf("storage: parse lock: %w", err)
	}
	st := LockStatus{Held: true, Lock: l}
	if m.TTL > 0 && time.Since(time.UnixMilli(l.AcquiredAt)) > m.TTL {
		st.Stale = true
	}
	return st, nil
}

// Acquire writes the lock for player (overwrites any existing lock).
func (m *LockManager) Acquire(ctx context.Context, worldGUID, player string) error {
	l := Lock{Player: player, AcquiredAt: time.Now().UnixMilli()}
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return m.Store.Put(ctx, LockKey(worldGUID), data)
}

// Release deletes the lock.
func (m *LockManager) Release(ctx context.Context, worldGUID string) error {
	err := m.Store.Delete(ctx, LockKey(worldGUID))
	if err == ErrNotFound {
		return nil
	}
	return err
}
