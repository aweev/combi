package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
)

func TestRedisIdempotencyStore(t *testing.T) {
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	store := NewRedisIdempotencyStore(rdb, "test:idemp:", logger)
	ctx := context.Background()

	key := "payment-123"

	t.Run("Store and retrieve success result", func(t *testing.T) {
		want := json.RawMessage(`{"tx":"ok"}`)
		if err := store.StoreResult(ctx, key, want, nil, 5*time.Second); err != nil {
			t.Fatal(err)
		}

		got, execErr, found, err := store.GetResult(ctx, key)
		if err != nil || !found || execErr != nil {
			t.Fatalf("unexpected: err=%v found=%v execErr=%v", err, found, execErr)
		}
		if !jsonEqual(got, want) {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("Store and retrieve error result", func(t *testing.T) {
		storeErr := errors.New("insufficient funds")
		if err := store.StoreResult(ctx, key+"-fail", nil, storeErr, 5*time.Second); err != nil {
			t.Fatal(err)
		}

		got, execErr, found, err := store.GetResult(ctx, key+"-fail")
		if err != nil || !found || execErr == nil {
			t.Fatalf("unexpected: err=%v found=%v execErr=%v", err, found, execErr)
		}
		if execErr.Error() != storeErr.Error() {
			t.Fatalf("got error %q, want %q", execErr, storeErr)
		}
		if got != nil {
			t.Fatalf("expected nil result for error case, got %s", got)
		}
	})

	t.Run("Not found", func(t *testing.T) {
		_, _, found, err := store.GetResult(ctx, "unknown-key")
		if err != nil || found {
			t.Fatalf("expected not found without error, got err=%v found=%v", err, found)
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		if err := store.StoreResult(ctx, "short", json.RawMessage(`{}`), nil, 1*time.Second); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond)
		_, _, found, _ := store.GetResult(ctx, "short")
		if found {
			t.Fatal("key should have expired")
		}
	})
}

func jsonEqual(a, b json.RawMessage) bool {
	// RawMessage is just []byte; nil and empty are equivalent for us
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return string(a) == string(b)
}
