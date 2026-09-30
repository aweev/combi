package rbac

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 403 Forbidden
	ErrAccessDenied = apperror.New(http.StatusForbidden, "ACCESS_DENIED", "You do not have the required permissions to perform this action.", nil)

	// 404 Not Found
	ErrPermissionNotFound     = apperror.New(http.StatusNotFound, "PERMISSION_NOT_FOUND", "The specified permission could not be found.", nil)
	ErrRoleNotFound           = apperror.New(http.StatusNotFound, "ROLE_NOT_FOUND", "The specified role could not be found.", nil)
	ErrRoleAssignmentNotFound = apperror.New(http.StatusNotFound, "ROLE_ASSIGNMENT_NOT_FOUND", "The specified user is not assigned to this role.", nil)
	ErrPolicyNotFound         = apperror.New(http.StatusNotFound, "POLICY_NOT_FOUND", "The specified policy could not be found.", nil)

	// 409 Conflict
	ErrPermissionAlreadyExists = apperror.New(http.StatusConflict, "PERMISSION_ALREADY_EXISTS", "A permission with this name already exists.", nil)
	ErrRoleAlreadyExists       = apperror.New(http.StatusConflict, "ROLE_ALREADY_EXISTS", "A role with this name or slug already exists.", nil)
	ErrRoleInUse               = apperror.New(http.StatusConflict, "ROLE_IN_USE", "This role is currently assigned to users and cannot be deleted.", nil)

	// 500 Internal Server Error (or 409 Conflict, depending on context)
	ErrSystemRoleImmutable   = apperror.New(http.StatusInternalServerError, "SYSTEM_ROLE_IMMUTABLE", "System roles cannot be modified or deleted via the API.", nil)
	ErrCircularRoleHierarchy = apperror.New(http.StatusInternalServerError, "CIRCULAR_HIERARCHY_DETECTED", "A circular role hierarchy was detected.", nil)
)
