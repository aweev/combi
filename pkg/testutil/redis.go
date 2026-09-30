package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestRedis starts a Redis container and returns a connected client and a teardown function.
func NewTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()

	// Use a context that respects the test's timeout.
	// If the test times out, this context will be cancelled,
	// which can help propagate the cancellation to testcontainers.
	ctx := context.Background()

	t.Log("Starting Redis container...")

	// Safely catch any panic from testcontainers (like missing Docker daemon) and skip the test
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Docker daemon is not running or accessible. Skipping test. Recovered panic: %v", r)
		}
	}()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(20 * time.Second), // Increased timeout slightly
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("failed to start redis container (skipping test). Is Docker running and accessible? Error: %s", err)
		return nil, func() {}
	}
	t.Log("Redis container started.")

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis container host: %s", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis container port: %s", err)
	}

	addr := fmt.Sprintf("%s:%s", host, port.Port())
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Add a timeout to the initial ping to prevent it from hanging indefinitely.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		t.Fatalf("failed to connect to test redis at %s: %s", addr, err)
	}

	t.Log("Test redis is ready at", addr)

	teardown := func() {
		t.Log("Shutting down test redis container...")
		_ = client.Close()
		teardownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := redisContainer.Terminate(teardownCtx); err != nil {
			// This is not a fatal test error, but it's good to know if cleanup fails.
			t.Logf("failed to terminate redis container gracefully: %s", err)
		}
		t.Log("Test redis container shut down.")
	}

	return client, teardown
}
