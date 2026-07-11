package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"
	"time"
)

// memStore is an in-memory Storage for testing.
type memStore struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func newMemStore() *memStore { return &memStore{objs: map[string][]byte{}} }

func (m *memStore) Upload(_ context.Context, key string, r io.Reader, size int64) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = b
	return nil
}

func (m *memStore) Download(_ context.Context, key string, w io.Writer, prog func(int64, int64)) error {
	m.mu.Lock()
	b, ok := m.objs[key]
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	if prog != nil {
		prog(0, int64(len(b)))
	}
	w.Write(b)
	if prog != nil {
		prog(int64(len(b)), int64(len(b)))
	}
	return nil
}

func (m *memStore) List(_ context.Context, prefix string) ([]Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Object
	for k, v := range m.objs {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			t, up, _ := ParseVersionKey(k)
			o := Object{Key: k, Size: int64(len(v)), LastModified: t}
			o.Uploader = up
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objs[key]; !ok {
		return ErrNotFound
	}
	delete(m.objs, key)
	return nil
}

func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), b...), nil
}

func (m *memStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = append([]byte(nil), data...)
	return nil
}

func TestVersions(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	guid := "WORLD123"
	k1, _ := UploadVersion(ctx, s, guid, "alice", bytes.NewReader([]byte("a")), 1)
	time.Sleep(2 * time.Millisecond)
	k2, _ := UploadVersion(ctx, s, guid, "bob", bytes.NewReader([]byte("b")), 1)

	latest, err := LatestVersion(ctx, s, guid)
	if err != nil {
		t.Fatal(err)
	}
	if latest != k2 {
		t.Errorf("latest = %s, want %s", latest, k2)
	}
	versions, _ := ListVersions(ctx, s, guid)
	if len(versions) != 2 || versions[0].Key != k2 {
		t.Errorf("versions = %+v", versions)
	}
	if versions[0].Uploader != "bob" {
		t.Errorf("uploader = %s", versions[0].Uploader)
	}
	// lock.json must not appear as a version.
	s.Put(ctx, LockKey(guid), []byte("{}"))
	versions, _ = ListVersions(ctx, s, guid)
	if len(versions) != 2 {
		t.Errorf("lock.json leaked into versions: %d", len(versions))
	}
	_ = k1
}

func TestLockManager(t *testing.T) {
	ctx := context.Background()
	s := newMemStore()
	lm := &LockManager{Store: s, TTL: time.Hour}

	st, err := lm.Status(ctx, "W")
	if err != nil {
		t.Fatal(err)
	}
	if st.Held {
		t.Error("lock should not be held initially")
	}

	if err := lm.Acquire(ctx, "W", "alice"); err != nil {
		t.Fatal(err)
	}
	st, _ = lm.Status(ctx, "W")
	if !st.Held || st.Lock.Player != "alice" {
		t.Errorf("after acquire: %+v", st)
	}
	if st.Stale {
		t.Error("fresh lock should not be stale")
	}

	// stale lock
	s.objs[LockKey("W")] = []byte(fmt.Sprintf(`{"player":"bob","acquired_at":%d}`, time.Now().Add(-2*time.Hour).UnixMilli()))
	st, _ = lm.Status(ctx, "W")
	if !st.Held || !st.Stale {
		t.Errorf("old lock should be stale: %+v", st)
	}

	if err := lm.Release(ctx, "W"); err != nil {
		t.Fatal(err)
	}
	st, _ = lm.Status(ctx, "W")
	if st.Held {
		t.Error("lock should be released")
	}
}
