// internal/domain/financial/permissions.go
package financial

import "github.com/combi/pkg/domain/core"

const (
	PermFinancialRead       core.PermissionName = "financial:read"
	PermFinancialManage     core.PermissionName = "financial:manage"
	PermFinancialRefund     core.PermissionName = "financial:refund"
	PermFinancialLedgerRead core.PermissionName = "financial:ledger:read"
)
