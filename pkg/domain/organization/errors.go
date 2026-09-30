package organization

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidInput     = apperror.New(http.StatusBadRequest, "INVALID_INPUT", "Invalid input provided.", nil)
	ErrCannotInviteSelf = apperror.New(http.StatusBadRequest, "CANNOT_INVITE_SELF", "You cannot send an invitation to yourself.", nil)

	// 403 Forbidden
	ErrPermissionDenied      = apperror.New(http.StatusForbidden, "PERMISSION_DENIED", "You do not have permission to perform this action in the organization.", nil)
	ErrMustBeOwner           = apperror.New(http.StatusForbidden, "MUST_BE_OWNER", "This action can only be performed by an organization owner.", nil)
	ErrMustBeAdmin           = apperror.New(http.StatusForbidden, "MUST_BE_ADMIN", "This action requires administrative privileges in the organization.", nil)
	ErrMaxMembersReached     = apperror.New(http.StatusForbidden, "MAX_MEMBERS_REACHED", "The organization has reached the maximum number of members for its current plan.", nil)
	ErrCannotChangeOwnerRole = apperror.New(http.StatusForbidden, "CANNOT_CHANGE_OWNER_ROLE", "The Owner role cannot be changed or removed from the last owner.", nil)
	ErrCannotRemoveLastOwner = apperror.New(http.StatusForbidden, "CANNOT_REMOVE_LAST_OWNER", "The last owner of an organization cannot be removed.", nil)
	ErrNotOrganizationMember = apperror.New(http.StatusForbidden, "NOT_ORGANIZATION_MEMBER", "The specified user is not a member of the organization.", nil)

	// 404 Not Found
	ErrOrganizationNotFound = apperror.New(http.StatusNotFound, "ORGANIZATION_NOT_FOUND", "The specified organization could not be found.", nil)
	ErrMemberNotFound       = apperror.New(http.StatusNotFound, "MEMBER_NOT_FOUND", "The specified member could not be found in this organization.", nil)
	ErrRoleNotFound         = apperror.New(http.StatusNotFound, "ROLE_NOT_FOUND", "The specified role could not be found in this organization.", nil)
	ErrInvitationNotFound   = apperror.New(http.StatusNotFound, "INVITATION_NOT_FOUND", "The specified invitation could not be found or has expired.", nil)

	// 409 Conflict
	ErrOrganizationAlreadyExists = apperror.New(http.StatusConflict, "ORGANIZATION_ALREADY_EXISTS", "An organization with this name or slug already exists.", nil)
	ErrUserIsAlreadyMember       = apperror.New(http.StatusConflict, "ALREADY_A_MEMBER", "The user is already a member of this organization.", nil)
	ErrRoleAlreadyExists         = apperror.New(http.StatusConflict, "ROLE_ALREADY_EXISTS", "A role with this name already exists in the organization.", nil)
	ErrInvitationAlreadySent     = apperror.New(http.StatusConflict, "INVITATION_ALREADY_SENT", "An invitation has already been sent to this email address.", nil)
	ErrInvitationAlreadyAccepted = apperror.New(http.StatusConflict, "INVITATION_ALREADY_ACCEPTED", "This invitation has already been accepted.", nil)
	ErrInvitationAlreadyDeclined = apperror.New(http.StatusConflict, "INVITATION_ALREADY_DECLINED", "This invitation has already been declined.", nil)
	ErrSystemRoleImmutable       = apperror.New(http.StatusConflict, "SYSTEM_ROLE_IMMUTABLE", "System-defined roles (e.g., Owner) cannot be modified or deleted.", nil)
	ErrDomainAlreadyVerified     = apperror.New(http.StatusConflict, "DOMAIN_ALREADY_VERIFIED", "This domain is already verified and claimed by another organization.", nil)

	// 410 Gone
	ErrInvitationExpired = apperror.New(http.StatusGone, "INVITATION_EXPIRED", "This invitation has expired.", nil)

	// 500 Internal Server Error
	ErrDomainVerificationFailed = apperror.New(http.StatusInternalServerError, "DOMAIN_VERIFICATION_FAILED", "The domain verification process failed.", nil)
	ErrCannotDeleteOrganization = apperror.New(http.StatusInternalServerError, "CANNOT_DELETE_ORGANIZATION", "The organization cannot be deleted at this time.", nil)
)
