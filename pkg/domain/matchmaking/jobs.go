package matchmaking

import "github.com/combi/pkg/domain/core"

const (
	// JobRunMatchingBatch runs the algorithm for all open requests.
	// Scheduled nightly or triggered manually.
	JobRunMatchingBatch core.JobType = "matchmaking:run_batch"

	// JobSyncCandidates ensures the 'MatchCandidate' table is up-to-date
	// with the latest User skills and Catalog offerings.
	JobSyncCandidates core.JobType = "matchmaking:sync_candidates"
)
