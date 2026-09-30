package circuit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/combi/pkg/config"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
)

func TestCircuitBreaker(t *testing.T) {
	rdb, teardown := testutil.NewTestRedis(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	cfg := config.CircuitBreakerConfig{
		Enabled:          true,
		FailureThreshold: 0.5, // 50 %
		SuccessThreshold: 2,   // 2 consecutive successes to close from half-open
		Interval:         10 * time.Second,
		Timeout:          1 * time.Second,
	}

	cb := NewCircuitBreaker("test", rdb, cfg, logger)
	ctx := context.Background()

	// helper counters
	var attempts, failures int

	t.Run("closed → allows all calls", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := cb.Execute(ctx, func() error { return nil })
			if err != nil {
				t.Fatalf("call %d: unexpected error %v", i, err)
			}
		}
	})

	t.Run("opens after failure threshold", func(t *testing.T) {
		attempts, failures = 0, 0
		// 4 calls: 3 failures → 75 % failure rate
		for i := 0; i < 4; i++ {
			_ = cb.Execute(ctx, func() error {
				attempts++
				if i < 3 {
					failures++
					return errors.New("boom")
				}
				return nil
			})
		}
		// should be open now
		st, _ := cb.GetState(ctx)
		if st != StateOpen {
			t.Fatalf("expected open, got %v", st)
		}
	})

	t.Run("open → fast fail without executing fn", func(t *testing.T) {
		executed := false
		err := cb.Execute(ctx, func() error { executed = true; return nil })
		if !errors.Is(err, ErrCircuitOpen) || executed {
			t.Fatalf("expected ErrCircuitOpen without execution, got err=%v executed=%v", err, executed)
		}
	})

	t.Run("half-open after timeout", func(t *testing.T) {
		// wait for the background goroutine to flip it
		time.Sleep(cfg.Timeout + 100*time.Millisecond)
		st, _ := cb.GetState(ctx)
		if st != StateHalfOpen {
			t.Fatalf("expected half-open, got %v", st)
		}
	})

	t.Run("half-open → closes on enough successes", func(t *testing.T) {
		// first success still half-open
		_ = cb.Execute(ctx, func() error { return nil })
		st, _ := cb.GetState(ctx)
		if st != StateHalfOpen {
			t.Fatalf("expected still half-open, got %v", st)
		}
		// second success → close
		_ = cb.Execute(ctx, func() error { return nil })
		st, _ = cb.GetState(ctx)
		if st != StateClosed {
			t.Fatalf("expected closed, got %v", st)
		}
	})

	t.Run("Reset forces closed and clears counters", func(t *testing.T) {
		// put it into open again quickly
		for i := 0; i < 3; i++ {
			_ = cb.Execute(ctx, func() error { return errors.New("fail") })
		}
		st, _ := cb.GetState(ctx)
		if st != StateOpen {
			t.Fatalf("expected open, got %v", st)
		}

		cb.Reset(ctx)
		st, _ = cb.GetState(ctx)
		if st != StateClosed {
			t.Fatalf("expected closed after reset, got %v", st)
		}
	})

	t.Run("disabled breaker always passes", func(t *testing.T) {
		disabledCfg := cfg
		disabledCfg.Enabled = false
		disabledCB := NewCircuitBreaker("disabled", rdb, disabledCfg, logger)

		wantErr := errors.New("ignored")
		err := disabledCB.Execute(ctx, func() error { return wantErr })

		if err == nil || err.Error() != wantErr.Error() {
			t.Fatalf("disabled breaker should propagate error, got %v", err)
		}
	})

	t.Run("Call delegates to Execute", func(t *testing.T) {
		executed := false
		err := cb.Call(ctx, "any-key", func() error { executed = true; return nil })
		if err != nil || !executed {
			t.Fatalf("Call did not delegate properly: err=%v executed=%v", err, executed)
		}
	})
}
