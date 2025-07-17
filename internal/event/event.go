//nolint:exhaustive
package event

const (
	TypeInformation = "info"
	TypeIncident    = "incident"
	TypeMaintenance = "maintenance"
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
const (
	// InfoPlanned is the status for planned information events.
	InfoPlanned   Status = "planned"
	InfoActive    Status = "active"
	InfoCompleted Status = "completed"
	InfoCancelled Status = "cancelled"
)

func IsMaintenanceStatus(status Status) bool {
	switch status {
	case MaintenancePlanned, MaintenanceInProgress, MaintenanceModified,
		MaintenanceCompleted, MaintenanceCancelled:
		return true
	}

	return false
}

// IsInformationStatus checks if a status is related to information events.
func IsInformationStatus(status Status) bool {
	switch status {
	case InfoPlanned, InfoActive, InfoCompleted, InfoCancelled:
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

const (
	maintenancePlannedText    = "Maintenance is planned."
	maintenanceInProgressText = "Maintenance is in progress."
	maintenanceCompletedText  = "Maintenance is completed."
	infoPlannedText           = "The informational notification is planned."
	infoActiveText            = "The informational notification is active."
	infoCompletedText         = "The informational notification is completed."
)

func MaintenancePlannedStatusText() string {
	return maintenancePlannedText
}

func MaintenanceInProgressStatusText() string {
	return maintenanceInProgressText
}

func MaintenanceCompletedStatusText() string {
	return maintenanceCompletedText
}

func InfoPlannedStatusText() string {
	return infoPlannedText
}

func InfoActiveStatusText() string {
	return infoActiveText
}

func InfoCompletedStatusText() string {
	return infoCompletedText
}
