package catalog

import (
	"github.com/combi/pkg/domain/core"
)

// Permissions
const (
	PermCatalogManage           core.PermissionName = "catalog:manage"             // Create/Edit Items
	PermCatalogCategoryManage   core.PermissionName = "catalog:category:manage"    // Manage Taxonomy
	PermCatalogApplicationRead  core.PermissionName = "catalog:application:read"   // View applications
	PermCatalogApplicationWrite core.PermissionName = "catalog:application:manage" // Approve/Reject
)

// Jobs (Background Tasks)
const (
	JobCatalogReindex core.JobType = "catalog:reindex" // Trigger full reindex of catalog to Search Service
)
