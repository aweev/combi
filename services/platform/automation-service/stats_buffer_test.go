package automation

import (
	"context"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	automation_mocks "github.com/combi/pkg/domain/automation/mocks"
	"github.com/combi/pkg/log"
)

func TestStatsBuffer_FlushSync(t *testing.T) {
	ctx := context.Background()
	db, mockRedis := redismock.NewClientMock()
	repo := automation_mocks.NewMockRepository(t)
	buffer := NewStatsBuffer(db, repo, &log.NoOpLogger{})
	ruleID := uuid.New()

	// 1. Get Dirty Rules
	mockRedis.ExpectSMembers("automation:stats:dirty_rules").SetVal([]string{ruleID.String()})

	// 2. Lua Script execution
	// Use the hash from the actual script variable to match what go-redis sends
	scriptSHA := statsFlushScript.Hash()

	mockRedis.ExpectEvalSha(scriptSHA, []string{"automation:stats:" + ruleID.String()}).SetVal([]interface{}{"10", "5"})

	// 3. Update Repo (This expectation previously failed because script run failed)
	repo.EXPECT().IncrementRuleStatsBatch(ctx, ruleID, int64(10), int64(5)).Return(nil).Once()

	// 4. Delete Dirty Set
	mockRedis.ExpectDel("automation:stats:dirty_rules").SetVal(1)

	err := buffer.FlushSync(ctx)
	require.NoError(t, err)
	require.NoError(t, mockRedis.ExpectationsWereMet())
}
