//nolint:exhaustive
package event

import (
	"fmt"
	"time"
)

const (
	TypeMaintenance = "maintenance"
	TypeIncident    = "incident"
	TypeInformation = "info"
)

type Status string

const OutDatedSystem Status = "SYSTEM"

const (
	MaintenancePlanned    Status = "planned"
	MaintenanceInProgress Status = "in progress"
	// MaintenanceModified is placed if the time window was changed.
	MaintenanceModified  Status = "modified"
	MaintenanceCompleted Status = "completed"
	MaintenanceCancelled Status = "cancelled"
)

func IsMaintenanceStatus(status Status) bool {
	switch status {
	case MaintenancePlanned, MaintenanceInProgress, MaintenanceModified,
		MaintenanceCompleted, MaintenanceCancelled:
		return true
	}

	return false
}

// Incident actions for opened incidents.
const (
	IncidentDetected      Status = "detected" // not implemented yet
	IncidentAnalysing     Status = "analysing"
	IncidentFixing        Status = "fixing"
	IncidentImpactChanged Status = "impact changed"
	IncidentObserving     Status = "observing"
	IncidentResolved      Status = "resolved"
)

func IsIncidentOpenStatus(status Status) bool {
	switch status {
	case IncidentDetected, IncidentAnalysing, IncidentFixing,
		IncidentImpactChanged, IncidentObserving, IncidentResolved:
		return true
	}

	return false
}

// These statuses are using only for closed incidents.
const (
	IncidentReopened Status = "reopened"
	// IncidentChanged indicates if the end date was changed for closed incident.
	IncidentChanged Status = "changed"
)

func IsIncidentClosedStatus(status Status) bool {
	switch status {
	case IncidentReopened, IncidentChanged:
		return true
	}

	return false
}

func MaintenancePlannedDescription(start, end time.Time) string {
	return fmt.Sprintf("Maintenance is planned from %s to %s.", start.Format(time.DateTime), end.Format(time.DateTime))
}
