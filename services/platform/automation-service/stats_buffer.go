package automation

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/combi/pkg/domain/automation"
	"github.com/combi/pkg/log"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// statsFlushScript is exported so tests can calculate its hash.
var statsFlushScript = redis.NewScript(`
	local t = redis.call('HGET', KEYS[1], 'total')
	local s = redis.call('HGET', KEYS[1], 'success')
	redis.call('DEL', KEYS[1])
	return {t, s}
`)

type StatsBuffer struct {
	redis  *redis.Client
	repo   automation.Repository
	logger log.Logger
}

func NewStatsBuffer(rdb *redis.Client, repo automation.Repository, logger log.Logger) *StatsBuffer {
	return &StatsBuffer{
		redis:  rdb,
		repo:   repo,
		logger: logger.With(log.String("component", "stats_buffer")),
	}
}

func (s *StatsBuffer) Increment(ctx context.Context, ruleID uuid.UUID, success bool) {
	pipe := s.redis.Pipeline()
	key := fmt.Sprintf("automation:stats:%s", ruleID.String())

	pipe.HIncrBy(ctx, key, "total", 1)
	if success {
		pipe.HIncrBy(ctx, key, "success", 1)
	}
	pipe.SAdd(ctx, "automation:stats:dirty_rules", ruleID.String())
	pipe.Expire(ctx, key, 24*time.Hour)

	if _, err := pipe.Exec(ctx); err != nil {
		s.logger.Warn("failed to buffer stats in redis", log.ErrorAttr(err))
	}
}

func (s *StatsBuffer) FlushSync(ctx context.Context) error {
	ruleIDs, err := s.redis.SMembers(ctx, "automation:stats:dirty_rules").Result()
	if err != nil {
		return err
	}
	if len(ruleIDs) == 0 {
		return nil
	}

	for _, ridStr := range ruleIDs {
		ruleID, err := uuid.Parse(ridStr)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("automation:stats:%s", ridStr)

		// Run the script using the exported variable
		res, err := statsFlushScript.Run(ctx, s.redis, []string{key}).Slice()
		if err != nil {
			s.logger.Error("stats flush lua script failed", err)
			continue
		}

		var totalDelta, successDelta int64

		// Safe parsing of Lua result
		if len(res) > 0 && res[0] != nil {
			if sVal, ok := res[0].(string); ok {
				totalDelta, _ = strconv.ParseInt(sVal, 10, 64)
			}
		}
		if len(res) > 1 && res[1] != nil {
			if sVal, ok := res[1].(string); ok {
				successDelta, _ = strconv.ParseInt(sVal, 10, 64)
			}
		}

		if totalDelta == 0 {
			continue
		}

		if err := s.repo.IncrementRuleStatsBatch(ctx, ruleID, totalDelta, successDelta); err != nil {
			s.logger.Error("failed to flush stats to db", err)
		}
	}

	s.redis.Del(ctx, "automation:stats:dirty_rules")
	return nil
}
