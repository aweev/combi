package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulerNotifier_Integration(t *testing.T) {
	db, dsn, teardown := testutil.NewTestDBWithDSN(t)
	defer teardown()

	logger := &log.NoOpLogger{}
	notifier := postgres.NewSchedulerNotifier(dsn, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Start Listening
	err := notifier.Listen(ctx)
	require.NoError(t, err)

	// Ensure listener is ready
	time.Sleep(100 * time.Millisecond)

	// 2. Trigger a notification via SQL
	scheduleID := "d27c2153-0b6c-4a0e-9554-2c1311d6a123"
	// We manually execute NOTIFY because our test setup might run migrations
	// but we want to test the mechanic explicitly.
	// In a real scenario, an INSERT to scheduled_jobs triggers this.
	_, err = db.ExecContext(ctx, "NOTIFY schedule_changes, '"+scheduleID+"'")
	require.NoError(t, err)

	// 3. Verify receipt
	select {
	case payload := <-notifier.Notifications():
		assert.Equal(t, scheduleID, payload)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}

	// 4. Test Trigger Logic (Real Table Insert)
	// This verifies the TRIGGER defined in the migration works.
	newScheduleID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
	insertQuery := `
		INSERT INTO scheduled_jobs (id, name, type, payload, queue, cron_expression) 
		VALUES ($1, 'Test Schedule', 'test.job', '{}', 'default', '* * * * *')
	`
	_, err = db.ExecContext(ctx, insertQuery, newScheduleID)
	require.NoError(t, err)

	select {
	case payload := <-notifier.Notifications():
		assert.Equal(t, newScheduleID, payload)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for trigger notification")
	}

	// 5. Graceful Close
	err = notifier.Close()
	assert.NoError(t, err)
}
