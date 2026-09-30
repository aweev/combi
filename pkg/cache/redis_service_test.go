package cache

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/testutil"
)

func TestRedisService(t *testing.T) {
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	svc := NewRedisService(rdb, "test:srv:")
	ctx := context.Background()

	t.Run("Set/Get bytes", func(t *testing.T) {
		key, val := "k1", []byte("hello")
		if err := svc.Set(ctx, key, val, 5*time.Second); err != nil {
			t.Fatal(err)
		}
		got, err := svc.Get(ctx, key)
		if err != nil || string(got) != string(val) {
			t.Fatalf("got %s err=%v", got, err)
		}
	})

	t.Run("GetObject/SetObject", func(t *testing.T) {
		type user struct{ Name string }
		key := "u1"
		if err := svc.Set(ctx, key, user{Name: "alice"}, 5*time.Second); err != nil {
			t.Fatal(err)
		}
		var u user
		found, err := svc.GetObject(ctx, key, &u)
		if err != nil || !found || u.Name != "alice" {
			t.Fatalf("unexpected: err=%v found=%v u=%+v", err, found, u)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "del"
		_ = svc.Set(ctx, key, "x", 5*time.Second)
		if err := svc.Delete(ctx, key); err != nil {
			t.Fatal(err)
		}
		if v, _ := svc.Get(ctx, key); v != nil {
			t.Fatal("key should be gone")
		}
	})

	t.Run("LRange/LPush", func(t *testing.T) {
		key := "list"
		pipe := svc.Pipeline()
		_ = pipe.LPush(ctx, key, "a", "b", "c")
		_ = pipe.LTrim(ctx, key, 0, 1) // keep c,b
		_ = pipe.Expire(ctx, key, 5*time.Second)
		if err := pipe.Exec(ctx); err != nil {
			t.Fatal(err)
		}
		vals, err := svc.LRange(ctx, key, 0, -1)
		if err != nil || len(vals) != 2 || vals[0] != "c" || vals[1] != "b" {
			t.Fatalf("unexpected list: %v err=%v", vals, err)
		}
	})

	t.Run("MGet", func(t *testing.T) {
		_ = svc.Set(ctx, "m1", "v1", 5*time.Second)
		_ = svc.Set(ctx, "m2", "v2", 5*time.Second)

		vals, err := svc.MGet(ctx, "m1", "m2", "missing")
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 3 {
			t.Fatalf("expected 3 values, got %d", len(vals))
		}

		// Redis returns a string for a string value
		if vals[0] != "v1" || vals[1] != "v2" || vals[2] != nil {
			t.Fatalf("wrong values: %v", vals)
		}
	})
}
