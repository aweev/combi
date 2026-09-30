package auth

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrOAuthProviderNotSupported = apperror.New(http.StatusBadRequest, "OAUTH_PROVIDER_NOT_SUPPORTED", "The specified OAuth provider is not supported.", nil)
	ErrOAuthStateMismatch        = apperror.New(http.StatusBadRequest, "OAUTH_STATE_MISMATCH", "OAuth state token mismatch (CSRF detected).", nil)
	ErrEmailAlreadyVerified      = apperror.New(http.StatusBadRequest, "EMAIL_ALREADY_VERIFIED", "This email address has already been verified.", nil)
	ErrSamePassword              = apperror.New(http.StatusBadRequest, "SAME_PASSWORD", "The new password cannot be the same as the old password.", nil)
	ErrInvalidTokenType          = apperror.New(http.StatusBadRequest, "INVALID_TOKEN_TYPE", "The provided token is not valid for this operation.", nil)
	ErrMfaNotEnabled             = apperror.New(http.StatusBadRequest, "MFA_NOT_ENABLED", "Multi-factor authentication is not enabled for this account.", nil)

	// 401 Unauthorized
	ErrUnauthorized        = apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required and has failed or has not yet been provided.", nil)
	ErrInvalidCredentials  = apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "The email or password provided is incorrect.", nil)
	ErrSessionNotFound     = apperror.New(http.StatusUnauthorized, "SESSION_NOT_FOUND", "The session could not be found.", nil)
	ErrSessionExpired      = apperror.New(http.StatusUnauthorized, "SESSION_EXPIRED", "The session has expired.", nil)
	ErrSessionRevoked      = apperror.New(http.StatusUnauthorized, "SESSION_REVOKED", "The session has been revoked.", nil)
	ErrTokenSignature      = apperror.New(http.StatusUnauthorized, "TOKEN_SIGNATURE_INVALID", "The token signature is invalid.", nil)
	ErrInvalidToken        = apperror.New(http.StatusUnauthorized, "TOKEN_INVALID", "The provided token is invalid.", nil)
	ErrInvalidMFACode      = apperror.New(http.StatusUnauthorized, "INVALID_MFA_CODE", "The multi-factor authentication code is invalid.", nil)
	ErrTokenNotFound       = apperror.New(http.StatusUnauthorized, "TOKEN_NOT_FOUND", "The requested token could not be found.", nil)
	ErrTokenExpired        = apperror.New(http.StatusUnauthorized, "TOKEN_EXPIRED", "The token has expired.", nil)
	ErrTokenAlreadyUsed    = apperror.New(http.StatusUnauthorized, "TOKEN_ALREADY_USED", "The token has already been used.", nil)
	ErrInvalidRefreshToken = apperror.New(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "The refresh token is invalid or expired.", nil)
	ErrIncorrectPassword   = apperror.New(http.StatusUnauthorized, "INCORRECT_PASSWORD", "The current password provided is incorrect.", nil)
	ErrBackupCodeInvalid   = apperror.New(http.StatusUnauthorized, "BACKUP_CODE_INVALID", "The provided backup code is invalid.", nil)
	ErrInvalidHashFormat   = apperror.New(http.StatusUnauthorized, "INVALID_HASH_FORMAT", "The hash is invalid", nil)

	// 403 Forbidden
	ErrForbidden          = apperror.New(http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource.", nil)
	ErrAccountSuspended   = apperror.New(http.StatusForbidden, "ACCOUNT_SUSPENDED", "This account has been suspended.", nil)
	ErrAccountNotVerified = apperror.New(http.StatusForbidden, "ACCOUNT_NOT_VERIFIED", "The account email has not been verified.", nil)
	ErrMfaRequired        = apperror.New(http.StatusForbidden, "MFA_REQUIRED", "Multi-factor authentication is required to complete this action.", nil)

	// 404 Not Found
	ErrAccountNotFound    = apperror.New(http.StatusNotFound, "ACCOUNT_NOT_FOUND", "An account with the specified criteria could not be found.", nil)
	ErrAuthMethodNotFound = apperror.New(http.StatusNotFound, "AUTH_METHOD_NOT_FOUND", "The requested authentication method could not be found for this user.", nil)

	// 409 Conflict
	ErrAccountExists              = apperror.New(http.StatusConflict, "ACCOUNT_EXISTS", "An account with this email address already exists.", nil)
	ErrAuthMethodExists           = apperror.New(http.StatusConflict, "AUTH_METHOD_EXISTS", "An authentication method of this type already exists for the user.", nil)
	ErrSocialAccountAlreadyLinked = apperror.New(http.StatusConflict, "SOCIAL_ACCOUNT_LINKED", "This social account is already linked to another user.", nil)
	ErrMfaAlreadyEnabled          = apperror.New(http.StatusConflict, "MFA_ALREADY_ENABLED", "Multi-factor authentication is already enabled for this account.", nil)

	// 422 Unprocessable Entity
	ErrPasswordTooWeak = apperror.New(http.StatusUnprocessableEntity, "PASSWORD_TOO_WEAK", "The password does not meet the security requirements.", nil)

	// 429 Too Many Requests
	ErrTooManyLoginAttempts = apperror.New(http.StatusTooManyRequests, "TOO_MANY_LOGIN_ATTEMPTS", "Too many failed login attempts; the account is temporarily locked.", nil)
	ErrRateLimitExceeded    = apperror.New(http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "You have made too many requests. Please try again later.", nil)
)
