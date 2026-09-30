package reporting

import "github.com/combi/pkg/domain/core"

const (
	// JobGenerateReport is the heavy lifter. It fetches data, renders HTML/CSV,
	// converts to PDF (if needed), uploads to Storage, and updates DB.
	JobGenerateReport core.JobType = "reporting:generate"

	// JobCleanupExpiredReports runs periodically to delete old files from storage
	// to save costs and reduce liability (retention policy).
	JobCleanupExpiredReports core.JobType = "reporting:cleanup"
)
