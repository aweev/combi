package reporting

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidParameters = apperror.New(http.StatusBadRequest, "INVALID_REPORT_PARAMS", "The provided parameters do not match the template schema.", nil)
	ErrUnsupportedFormat = apperror.New(http.StatusBadRequest, "UNSUPPORTED_FORMAT", "This template does not support the requested output format.", nil)

	// 404 Not Found
	ErrTemplateNotFound = apperror.New(http.StatusNotFound, "TEMPLATE_NOT_FOUND", "The specified report template could not be found.", nil)
	ErrReportNotFound   = apperror.New(http.StatusNotFound, "REPORT_NOT_FOUND", "The requested report could not be found.", nil)

	// 500 Internal
	ErrGenerationFailed = apperror.New(http.StatusInternalServerError, "REPORT_GENERATION_FAILED", "Failed to generate the report file.", nil)
)
