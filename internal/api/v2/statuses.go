package v2

const (
	MaintenancePlanned    = "planned"
	MaintenanceInProgress = "in progress"
	// MaintenanceModified is placed if the time window was changed.
	MaintenanceModified  = "modified"
	MaintenanceCompleted = "completed"
	MaintenanceCancelled = "cancelled"
)

//nolint:gochecknoglobals
var maintenanceStatuses = map[string]struct{}{
	MaintenancePlanned:    {},
	MaintenanceInProgress: {},
	MaintenanceModified:   {},
	MaintenanceCompleted:  {},
	MaintenanceCancelled:  {},
}

// Incident actions for opened incidents.
const (
	IncidentDetected      = "detected" // not implemented yet
	IncidentAnalysing     = "analysing"
	IncidentFixing        = "fixing"
	IncidentImpactChanged = "impact changed"
	IncidentObserving     = "observing"
	IncidentResolved      = "resolved"
)

//nolint:gochecknoglobals
var incidentOpenStatuses = map[string]struct{}{
	IncidentDetected:      {},
	IncidentAnalysing:     {},
	IncidentFixing:        {},
	IncidentImpactChanged: {},
	IncidentObserving:     {},
	IncidentResolved:      {},
}

// These statuses are using only for closed incidents.
const (
	IncidentReopened = "reopened"
	// IncidentChanged indicates if the end date was changed for closed incident.
	IncidentChanged = "changed"
)

//nolint:gochecknoglobals
var incidentClosedStatuses = map[string]struct{}{
	IncidentReopened: {},
	IncidentChanged:  {},
}
