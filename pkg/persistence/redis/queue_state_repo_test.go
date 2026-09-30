package redis

import (
	"context"
	"testing"

	"github.com/combi/pkg/testutil"
)

func TestRedisQueueStateRepository(t *testing.T) {
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	repo := NewQueueStateRepository(rdb)
	ctx := context.Background()

	t.Run("Pause / IsPaused / Resume flow", func(t *testing.T) {
		queue := "emails"

		// not paused initially
		paused, err := repo.IsQueuePaused(ctx, queue)
		if err != nil || paused {
			t.Fatalf("IsPaused: err=%v paused=%v (want false)", err, paused)
		}

		// pause
		if err := repo.PauseQueue(ctx, queue); err != nil {
			t.Fatalf("Pause: %v", err)
		}

		paused, err = repo.IsQueuePaused(ctx, queue)
		if err != nil || !paused {
			t.Fatalf("IsPaused: err=%v paused=%v (want true)", err, paused)
		}

		// resume
		if err := repo.ResumeQueue(ctx, queue); err != nil {
			t.Fatalf("Resume: %v", err)
		}

		paused, err = repo.IsQueuePaused(ctx, queue)
		if err != nil || paused {
			t.Fatalf("IsPaused: err=%v paused=%v (want false)", err, paused)
		}
	})

	t.Run("GetPaused lists all paused queues", func(t *testing.T) {
		// clean slate
		_ = rdb.Del(ctx, pausedSetKey)

		queues := []string{"q1", "q2", "q3"}
		for _, q := range queues {
			if err := repo.PauseQueue(ctx, q); err != nil {
				t.Fatal(err)
			}
		}

		list, err := repo.GetPausedQueues(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Redis Set → order not guaranteed
		want := make(map[string]struct{})
		for _, q := range queues {
			want[q] = struct{}{}
		}
		got := make(map[string]struct{})
		for _, q := range list {
			got[q] = struct{}{}
		}

		if len(got) != len(want) {
			t.Fatalf("GetPaused: got %v, want %v", got, want)
		}
		for k := range want {
			if _, ok := got[k]; !ok {
				t.Fatalf("GetPaused: missing %q", k)
			}
		}
	})
}
