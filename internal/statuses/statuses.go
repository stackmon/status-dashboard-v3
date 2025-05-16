//nolint:exhaustive
package statuses

import (
	"fmt"
	"time"
)

type EventStatus string

const OutDatedSystem EventStatus = "SYSTEM"

const (
	MaintenancePlanned    EventStatus = "planned"
	MaintenanceInProgress EventStatus = "in progress"
	// MaintenanceModified is placed if the time window was changed.
	MaintenanceModified  EventStatus = "modified"
	MaintenanceCompleted EventStatus = "completed"
	MaintenanceCancelled EventStatus = "cancelled"
)

func IsMaintenanceStatus(status EventStatus) bool {
	switch status {
	case MaintenancePlanned, MaintenanceInProgress, MaintenanceModified,
		MaintenanceCompleted, MaintenanceCancelled:
		return true
	}

	return false
}

// Incident actions for opened incidents.
const (
	IncidentDetected      EventStatus = "detected" // not implemented yet
	IncidentAnalysing     EventStatus = "analysing"
	IncidentFixing        EventStatus = "fixing"
	IncidentImpactChanged EventStatus = "impact changed"
	IncidentObserving     EventStatus = "observing"
	IncidentResolved      EventStatus = "resolved"
)

func IsIncidentOpenStatus(status EventStatus) bool {
	switch status {
	case IncidentDetected, IncidentAnalysing, IncidentFixing,
		IncidentImpactChanged, IncidentObserving, IncidentResolved:
		return true
	}

	return false
}

// These statuses are using only for closed incidents.
const (
	IncidentReopened EventStatus = "reopened"
	// IncidentChanged indicates if the end date was changed for closed incident.
	IncidentChanged EventStatus = "changed"
)

func IsIncidentClosedStatus(status EventStatus) bool {
	switch status {
	case IncidentReopened, IncidentChanged:
		return true
	}

	return false
}

func MaintenancePlannedDescription(start, end time.Time) string {
	return fmt.Sprintf("Maintenance is planned from %s to %s.", start.Format(time.DateTime), end.Format(time.DateTime))
}
