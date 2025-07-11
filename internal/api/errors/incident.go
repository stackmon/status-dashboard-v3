package errors

import "errors"

var ErrIncidentDSNotExist = errors.New("incident does not exist")
var ErrIncidentEndDateShouldBeEmpty = errors.New("incident end_date should be empty")
var ErrIncidentStartDateInFuture = errors.New("incident start_date should not be in the future")
var ErrIncidentUpdatesShouldBeEmpty = errors.New("incident updates should be empty")
var ErrIncidentTypeImpactMismatch = errors.New(
	"impact must be 0 for type 'maintenance' or 'info' and gt 0 for 'incident'",
)
var ErrIncidentInvalidType = errors.New("incident type is invalid, must be 'maintenance' 'info' or 'incident'")

var ErrIncidentCreationMaintenanceExists = errors.New("incident creation failed, component in maintenance")
var ErrIncidentCreationLowImpact = errors.New(
	"incident creation failed, exists the incident with higher impact for component",
)
var ErrIncidentFQueryInvalidFormat = errors.New("incident filter query parameter has an invalid format or value")

// Errors for patching incident

var ErrIncidentPatchMaintenanceImpactForbidden = errors.New("can not change impact for maintenance")
var ErrIncidentPatchMaintenanceStatus = errors.New("wrong status for maintenance")
var ErrIncidentPatchStatus = errors.New("wrong status for incident")
var ErrIncidentPatchClosedStatus = errors.New("wrong status for closed incident")
var ErrIncidentPatchOpenedStartDate = errors.New("can not change start date for open incident")
var ErrIncidentPatchOpenedEndDateMissing = errors.New("wrong end date with resolved status")
var ErrIncidentPatchImpactStatusWrong = errors.New("wrong status for changing impact")
var ErrIncidentPatchImpactToMaintenanceForbidden = errors.New("can not change impact to maintenance")

var ErrMaintenanceEndDateEmpty = errors.New("maintenance end_date is empty")
