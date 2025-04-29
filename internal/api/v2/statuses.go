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
	IncidentAnalysing     = "analysing"
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

// IsValidMaintenanceStatus checks if the status is valid for maintenance
func IsValidIncidentFilterStatus(status string) bool {
	if _, ok := maintenanceStatuses[status]; ok {
		return true
	}
	if _, ok := incidentOpenStatuses[status]; ok {
		return true
	}
	if _, ok := incidentClosedStatuses[status]; ok {
		return true
	}
	return false
}
