// internal/persistence/postgres/monitoring_repository_test.go
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/pkg/monitoring"
	"github.com/combi/pkg/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitoringRepository(t *testing.T) {
	db, teardown := testutil.NewTestDB(t)
	defer teardown()

	repo := postgres.NewMonitoringRepository(db, &log.NoOpLogger{})
	ctx := context.Background()

	t.Run("AlertRule_Lifecycle", func(t *testing.T) {
		rule := &monitoring.AlertRule{
			ID:          uuid.New(),
			Name:        "HighCPU",
			Description: "Triggers when CPU > 90%",
			IsEnabled:   true,
			Severity:    monitoring.SeverityCritical,
			PromQLQuery: "avg(rate(cpu_usage[5m])) > 0.9",
			ForDuration: 5 * time.Minute,
			Labels:      map[string]interface{}{"team": "infra"},
			Annotations: map[string]string{"summary": "High CPU Usage"},
		}

		// 1. Create
		require.NoError(t, repo.CreateRule(ctx, rule))

		// 2. Get By ID
		got, err := repo.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.Equal(t, rule.Name, got.Name)
		assert.Equal(t, rule.ForDuration, got.ForDuration)
		assert.Equal(t, "infra", got.Labels["team"])

		// 3. Get By Name
		gotByName, err := repo.GetRuleByName(ctx, "HighCPU")
		require.NoError(t, err)
		assert.Equal(t, rule.ID, gotByName.ID)

		// 4. List
		list, err := repo.ListRules(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 1)

		// 5. Update
		rule.IsEnabled = false
		rule.Severity = monitoring.SeverityWarning
		rule.ForDuration = 10 * time.Minute
		require.NoError(t, repo.UpdateRule(ctx, rule))

		updated, err := repo.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.False(t, updated.IsEnabled)
		assert.Equal(t, monitoring.SeverityWarning, updated.Severity)
		assert.Equal(t, 10*time.Minute, updated.ForDuration)

		// 6. Delete
		require.NoError(t, repo.DeleteRule(ctx, rule.ID))
		_, err = repo.GetRule(ctx, rule.ID)
		assert.ErrorIs(t, err, monitoring.ErrAlertRuleNotFound)
	})

	t.Run("AlertRule_NameConflict", func(t *testing.T) {
		r1 := &monitoring.AlertRule{
			ID:          uuid.New(),
			Name:        "UniqueRule",
			Severity:    monitoring.SeverityInfo,
			PromQLQuery: "up == 1",
			ForDuration: time.Minute,
		}
		require.NoError(t, repo.CreateRule(ctx, r1))

		r2 := &monitoring.AlertRule{
			ID:          uuid.New(),
			Name:        "UniqueRule", // Same name
			Severity:    monitoring.SeverityInfo,
			PromQLQuery: "up == 0",
			ForDuration: time.Minute,
		}
		err := repo.CreateRule(ctx, r2)
		assert.ErrorIs(t, err, monitoring.ErrAlertRuleNameTaken)
	})

	t.Run("Silence_Lifecycle", func(t *testing.T) {
		now := time.Now().UTC()

		// Create active silence
		activeSilence := &monitoring.AlertSilence{
			ID:        uuid.New(),
			Matchers:  map[string]interface{}{"alertname": "DatabaseDown"},
			StartsAt:  now.Add(-1 * time.Hour), // Started an hour ago
			EndsAt:    now.Add(1 * time.Hour),  // Ends in an hour
			Comment:   "Maintenance window",
			CreatedBy: "admin@ngo-platform.org",
		}
		require.NoError(t, repo.CreateSilence(ctx, activeSilence))

		// Create expired silence
		expiredSilence := &monitoring.AlertSilence{
			ID:        uuid.New(),
			Matchers:  map[string]interface{}{"alertname": "OldAlert"},
			StartsAt:  now.Add(-5 * time.Hour),
			EndsAt:    now.Add(-4 * time.Hour),
			Comment:   "Old maintenance",
			CreatedBy: "system",
		}
		require.NoError(t, repo.CreateSilence(ctx, expiredSilence))

		// Create future silence
		futureSilence := &monitoring.AlertSilence{
			ID:        uuid.New(),
			Matchers:  map[string]interface{}{"alertname": "FutureAlert"},
			StartsAt:  now.Add(24 * time.Hour),
			EndsAt:    now.Add(25 * time.Hour),
			Comment:   "Future upgrade",
			CreatedBy: "admin",
		}
		require.NoError(t, repo.CreateSilence(ctx, futureSilence))

		// 1. Get
		got, err := repo.GetSilence(ctx, activeSilence.ID)
		require.NoError(t, err)
		assert.Equal(t, "Maintenance window", got.Comment)
		assert.Equal(t, "DatabaseDown", got.Matchers["alertname"])

		// 2. List All (should return 3)
		all, err := repo.ListAllSilences(ctx)
		require.NoError(t, err)
		assert.Len(t, all, 3)

		// 3. List Active (should return only 1)
		// Note: ListActiveSilences typically returns silences that overlap with 'now'.
		// Logic: Start <= now AND End > now
		active, err := repo.ListActiveSilences(ctx, now)
		require.NoError(t, err)
		assert.Len(t, active, 1)
		assert.Equal(t, activeSilence.ID, active[0].ID)

		// 4. Delete
		require.NoError(t, repo.DeleteSilence(ctx, activeSilence.ID))
		_, err = repo.GetSilence(ctx, activeSilence.ID)
		assert.ErrorIs(t, err, monitoring.ErrAlertSilenceNotFound)
	})
}
