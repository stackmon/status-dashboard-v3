package v1

// TODO: move these statuses to consts and use it
//
//nolint:unused,gochecknoglobals
var stasuses = ` 
MAINTENANCE_STATUSES = {
"in progress": "Maintenance is in progress",
"modified": "Maintenance time window has been modified",
"completed": "Maintenance is successfully completed",
}

INCIDENT_STATUSES = {
"analyzing": "Analyzing incident (problem not known yet)",
"fixing": "Fixing incident (problem identified, working on fix)",
"impact changed": "Impact changed (incident impact has been changed)",
"observing": "Observing fix (fix deployed, watching recovery)",
"resolved": "Incident Resolved (service is fully available. Done)",
}

INCIDENT_ACTIONS = {
"reopened": "Incident reopened (resolved incident has ben reopened)",
"changed": "Incident changed: (end date has been changed)",
}
`
