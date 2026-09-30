// audit-log-service — module-level errors for not-yet-mounted dependencies.
//
// The audit module consumes several platform services that are not ported
// to the combi chassis yet (queue dispatcher, privacy manager). Those
// dependencies arrive nil from the fx graph (dig `optional:"true"` params);
// instead of panicking on a nil interface, the service layer returns these
// 503s so callers get an honest "not available in this host yet" signal.
package auditlog

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// ErrDispatcherNotConfigured is returned by operations that need to
	// dispatch background jobs when no queue host is mounted yet.
	ErrDispatcherNotConfigured = apperror.New(http.StatusServiceUnavailable, "AUDIT_DISPATCHER_NOT_CONFIGURED", "The queue dispatcher is not configured for the audit service.", nil)

	// ErrPrivacyManagerNotConfigured is returned by compliance operations
	// that need the privacy manager when it is not mounted yet.
	ErrPrivacyManagerNotConfigured = apperror.New(http.StatusServiceUnavailable, "AUDIT_PRIVACY_NOT_CONFIGURED", "The privacy manager is not configured for the audit service.", nil)
)
