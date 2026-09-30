package campaign

import (
	"net/http"

	"github.com/combi/pkg/apperror"
)

var (
	// 400 Bad Request
	ErrInvalidGoal      = apperror.New(http.StatusBadRequest, "INVALID_GOAL", "Campaign goal must be greater than zero.", nil)
	ErrCampaignClosed   = apperror.New(http.StatusBadRequest, "CAMPAIGN_CLOSED", "This campaign is no longer accepting donations.", nil)
	ErrInvalidDateRange = apperror.New(http.StatusBadRequest, "INVALID_DATE_RANGE", "Start date must be before end date.", nil)

	// 404 Not Found
	ErrCampaignNotFound   = apperror.New(http.StatusNotFound, "CAMPAIGN_NOT_FOUND", "The requested campaign could not be found.", nil)
	ErrFundraiserNotFound = apperror.New(http.StatusNotFound, "FUNDRAISER_NOT_FOUND", "The requested peer-to-peer fundraiser page could not be found.", nil)

	// 409 Conflict
	ErrSlugTaken = apperror.New(http.StatusConflict, "SLUG_TAKEN", "This URL slug is already in use.", nil)
)
