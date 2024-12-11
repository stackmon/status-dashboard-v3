package v2

const (
	MaintenanceInProgress = "in progress"
	// MaintenanceModified is placed if the time window was changed.
	MaintenanceModified  = "modified"
	MaintenanceCompleted = "completed"
)

//nolint:gochecknoglobals
var maintenanceStatuses = map[string]struct{}{
	MaintenanceInProgress: {},
	MaintenanceModified:   {},
	MaintenanceCompleted:  {},
}

// Incident actions for opened incidents.
const (
	IncidentAnalysing     = "analyzing"
	IncidentFixing        = "fixing"
	IncidentImpactChanged = "impact changed"
	IncidentObserving     = "observing"
	IncidentResolved      = "resolved"
)

//nolint:gochecknoglobals
var incidentOpenStatuses = map[string]struct{}{
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
