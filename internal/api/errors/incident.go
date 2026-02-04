package errors

import "errors"

var ErrIncidentDSNotExist = errors.New("event does not exist")
var ErrIncidentEndDateShouldBeEmpty = errors.New("event end_date should be empty")
var ErrIncidentStartDateInFuture = errors.New("event start_date should not be in the future")
var ErrIncidentUpdatesShouldBeEmpty = errors.New("event updates should be empty")
var ErrIncidentTypeImpactMismatch = errors.New(
	"impact must be 0 for type 'maintenance' or 'info' and gt 0 for 'incident'",
)
var ErrIncidentInvalidType = errors.New("event type is invalid, must be 'maintenance' 'info' or 'incident'")

// Errors for creating incident

var ErrIncidentSystemCreationWrongType = errors.New("system incident must be of type 'incident'")
var ErrIncidentCreationMaintenanceExists = errors.New("event creation failed, component in maintenance")
var ErrIncidentCreationLowImpact = errors.New(
	"incident creation failed, exists the incident with higher impact for component",
)

var ErrIncidentFQueryInvalidFormat = errors.New("filter query parameter has an invalid format or value")

// Errors for patching incident

var ErrIncidentPatchMaintenanceImpactForbidden = errors.New("can not change impact for maintenance")
var ErrIncidentPatchMaintenanceStatus = errors.New("wrong status for maintenance")
var ErrIncidentPatchInfoStatus = errors.New("wrong status for info event")
var ErrIncidentPatchIncidentStatus = errors.New("wrong status for incident")
var ErrIncidentPatchClosedStatus = errors.New("wrong status for closed incident")
var ErrIncidentPatchOpenedStartDate = errors.New("can not change start date for open incident")
var ErrIncidentPatchOpenedEndDateMissing = errors.New("wrong end date with resolved status")
var ErrIncidentPatchImpactStatusWrong = errors.New("wrong status for changing impact")
var ErrIncidentPatchImpactToZeroForbidden = errors.New("can not change impact to 0")

var ErrMaintenanceEndDateEmpty = errors.New("maintenance end_date is empty")

var ErrUpdateTextEmpty = errors.New("text field is required")
var ErrUpdateDSNotExist = errors.New("update does not exist")

// Errors for maintenance validation

var ErrMaintenanceContactEmailRequired = errors.New("contact_email is required for maintenance")
var ErrMaintenanceContactEmailInvalid = errors.New("contact_email has invalid format")
var ErrMaintenanceStartDateInPast = errors.New("start_date must be in the future for maintenance")
var ErrMaintenanceEndDateBeforeStart = errors.New("end_date must be after start_date")
var ErrMaintenanceDescriptionRequired = errors.New("description is required for maintenance")
