package errors

import "errors"

var ErrIncidentDSNotExist = errors.New("incident does not exist")
var ErrIncidentEndDateShouldBeEmpty = errors.New("incident end_date should be empty")
var ErrIncidentUpdatesShouldBeEmpty = errors.New("incident updates should be empty")

var ErrIncidentCreationMaintenanceExists = errors.New("incident creation failed, component in maintenance")
var ErrIncidentCreationLowImpact = errors.New(
	"incident creation failed, exists the incident with higher impact for component",
)
var ErrIncidentFTypeInvalidFormat = errors.New("incident filter 'type' invalid format")
var ErrIncidentFOpenedInvalidFormat = errors.New("incident filter 'opened' invalid format")
var ErrIncidentFDateInvalidFormat = errors.New("incident filter 'date' invalid format")
var ErrIncidentFImpactInvalidFormat = errors.New("incident filter 'impact' invalid format")
var ErrIncidentFSystemInvalidFormat = errors.New("incident filter 'system' invalid format")
var ErrIncidentFCompsInvalidFormat = errors.New("incident filter 'components' invalid format")
var ErrIncidentFStatusInvalidFormat = errors.New("invalid format or value for 'status' filter")

// Errors for patching incident

var ErrIncidentPatchMaintenanceImpactForbidden = errors.New("can not change impact for maintenance")
var ErrIncidentPatchMaintenanceStatus = errors.New("wrong status for maintenance")
var ErrIncidentPatchStatus = errors.New("wrong status for incident")
var ErrIncidentPatchClosedStatus = errors.New("wrong status for closed incident")
var ErrIncidentPatchOpenedStartDate = errors.New("can not change start date for open incident")
var ErrIncidentPatchOpenedEndDateMissing = errors.New("wrong end date with resolved status")
var ErrIncidentPatchImpactStatusWrong = errors.New("wrong status for changing impact")
var ErrIncidentPatchImpactToMaintenanceForbidden = errors.New("can not change impact to maintenance")
