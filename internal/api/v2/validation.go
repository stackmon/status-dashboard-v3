package v2

import (
	"math"
	"strconv"
	"strings"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

// IsValidIncidentFilterStatus checks if the status is valid for maintenance or incidents.
func IsValidIncidentFilterStatus(status event.Status) bool {
	if event.IsMaintenanceStatus(status) {
		return true
	}
	if event.IsIncidentOpenStatus(status) {
		return true
	}
	if event.IsIncidentClosedStatus(status) {
		return true
	}
	return false
}

// validateAndSetStatus validates the query status and sets it on db.IncidentsParams.
func validateAndSetStatus(queryStatus *event.Status, params *db.IncidentsParams) error {
	if queryStatus != nil {
		if !IsValidIncidentFilterStatus(*queryStatus) {
			return apiErrors.ErrIncidentFQueryInvalidFormat
		}
		params.Status = queryStatus
	}
	return nil
}

// parseAndSetComponents parses component IDs from a comma-separated string and sets them on db.IncidentsParams.
func parseAndSetComponents(queryComponents *string, params *db.IncidentsParams) error {
	if queryComponents != nil && *queryComponents != "" {
		compIDStrings := strings.Split(*queryComponents, ",")
		parsedComponentIDs := make([]int, 0, len(compIDStrings))

		for _, idStr := range compIDStrings {
			trimmedIDStr := strings.TrimSpace(idStr)
			idUint64, err := strconv.ParseUint(trimmedIDStr, 10, 64)
			if err != nil || idUint64 <= 0 || idUint64 > math.MaxInt32 {
				return apiErrors.ErrIncidentFQueryInvalidFormat
			}
			parsedComponentIDs = append(parsedComponentIDs, int(idUint64))
		}
		if len(parsedComponentIDs) > 0 {
			params.ComponentIDs = parsedComponentIDs
		}
	}
	return nil
}
