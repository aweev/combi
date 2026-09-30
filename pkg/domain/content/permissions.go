// internal/domain/content/permissions.go
package content

import "github.com/combi/pkg/domain/core"

const (
	// --- User/Editor Permissions ---
	// Allows reading entries (e.g., for delivery to a website).
	PermContentRead core.PermissionName = "content:entry:read"
	// Allows creating, updating, and managing entries.
	PermContentEdit core.PermissionName = "content:entry:edit"
	// Allows publishing and unpublishing entries.
	PermContentPublish core.PermissionName = "content:entry:publish"

	// --- Admin Permissions ---
	// A blanket permission for managing the entire content structure (Spaces and Models).
	PermContentAdmin core.PermissionName = "content:admin:manage"
)
