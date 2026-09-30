package auth

import "github.com/combi/pkg/domain/core"

const (
	// JobAuthCleanupExpiredTokens runs periodically to delete expired single-use tokens.
	JobAuthCleanupExpiredTokens core.JobType = "auth:cleanup_expired_tokens"
	// JobAuthCleanupExpiredSessions runs periodically to delete expired sessions.
	JobAuthCleanupExpiredSessions core.JobType = "auth:cleanup_expired_sessions"
)
