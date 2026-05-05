package bot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// newTestStore spins up a miniredis server and returns a Store + cleanup func.
func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewStore(rdb), mr
}

func TestStore_SaveAndLoad(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"package_code": "premium_pro_200"})
	want := State{
		Name: "buy_confirming",
		Data: json.RawMessage(payload),
	}

	if err := store.Save(ctx, 12345, want, 30*time.Minute); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(ctx, 12345)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
	if string(got.Data) != string(want.Data) {
		t.Errorf("Data: got %s, want %s", got.Data, want.Data)
	}
	if got.ExpiresAt == 0 {
		t.Error("ExpiresAt should be set after Save")
	}
}

func TestStore_LoadMissing(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.Load(ctx, 99999)
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !errors.Is(err, goredis.Nil) {
		t.Errorf("expected wrapped goredis.Nil, got: %v", err)
	}
}

func TestStore_Clear(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	st := State{Name: "awaiting_contact"}
	if err := store.Save(ctx, 42, st, time.Minute); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Clear(ctx, 42); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	_, err := store.Load(ctx, 42)
	if !errors.Is(err, goredis.Nil) {
		t.Errorf("after Clear, expected goredis.Nil, got: %v", err)
	}
}

func TestStore_ClearMissing(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	// Clear on a non-existent key must not error.
	if err := store.Clear(ctx, 77777); err != nil {
		t.Errorf("Clear on missing key should be no-op, got: %v", err)
	}
}

func TestStore_TTLExpiry(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	st := State{Name: "topup_waiting"}
	if err := store.Save(ctx, 555, st, 2*time.Second); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Fast-forward miniredis clock past TTL.
	mr.FastForward(3 * time.Second)

	_, err := store.Load(ctx, 555)
	if !errors.Is(err, goredis.Nil) {
		t.Errorf("after TTL expiry, expected goredis.Nil, got: %v", err)
	}
}

func TestStore_SaveZeroTTL(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	err := store.Save(ctx, 1, State{Name: "idle"}, 0)
	if err == nil {
		t.Fatal("expected error for zero TTL, got nil")
	}
}
