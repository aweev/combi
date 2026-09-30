// internal/domain/content/jobs.go
package content

import "github.com/combi/pkg/domain/core"

const (
	// JobContentReindexEntry triggers re-indexing of an entry in the search service after an update.
	JobContentReindexEntry core.JobType = "content:reindex_entry"
	// JobContentPublishScheduledEntry handles the publishing of an entry at a future time.
	JobContentPublishScheduledEntry core.JobType = "content:publish_scheduled_entry"
)
