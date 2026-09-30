package reporting

import "github.com/combi/pkg/domain/core"

const (
	// PermReportGenerate allows requesting reports.
	PermReportGenerate core.PermissionName = "report:generate"

	// PermTemplateManage allows creating/editing report templates.
	PermTemplateManage core.PermissionName = "report:template:manage"
)
