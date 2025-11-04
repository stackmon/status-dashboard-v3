package v2

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

const (
	defaultIncidentLimit = 50
	defaultPageNumber    = 1
)

// Event IDs and core data structures.
type IncidentID struct {
	ID int `json:"id" uri:"incidentID" binding:"required,gte=0"`
}

type IncidentData struct {
	Title string `json:"title" binding:"required"`
	//TODO: this field only valid for incident creation (legacy), but it should be an additional field in DB.
	Description string `json:"description,omitempty"`
	//    INCIDENT_IMPACTS = {
	//        0: Impact(0, "maintenance", "Scheduled maintenance", "info"),
	//        1: Impact(1, "minor", "Minor incident (i.e. performance impact)"),
	//        2: Impact(2, "major", "Major incident"),
	//        3: Impact(3, "outage", "Service outage"),
	//    }
	Impact     *int  `json:"impact" binding:"required,gte=0,lte=3"`
	Components []int `json:"components" binding:"required"`
	// Datetime format is standard: "2006-01-01T12:00:00Z"
	StartDate time.Time  `json:"start_date" binding:"required"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	System    *bool      `json:"system,omitempty"`
	//    Types of incidents:
	//    1. maintenance
	//    2. info
	//    3. incident
	// Type field is mandatory.
	Type    string            `json:"type" binding:"required,oneof=maintenance info incident"`
	Updates []EventUpdateData `json:"updates,omitempty"`
	// Status does not take into account OutDatedSystem status.
	Status event.Status `json:"status,omitempty"`
}

type Incident struct {
	IncidentID
	IncidentData
}

type APIGetIncidentsQuery struct {
	Types      *string       `form:"type" binding:"omitempty"` // custom validation in parseAndSetTypes
	IsActive   *bool         `form:"active" binding:"omitempty"`
	Status     *event.Status `form:"status"` // custom validation in validateAndSetStatus
	StartDate  *time.Time    `form:"start_date" binding:"omitempty"`
	EndDate    *time.Time    `form:"end_date" binding:"omitempty"`
	Impact     *int          `form:"impact" binding:"omitempty,gte=0,lte=3"`
	System     *bool         `form:"system" binding:"omitempty"`
	Components *string       `form:"components"` // custom validation in parseAndSetComponents
	Page       *int          `form:"page" binding:"omitempty,gte=1"`
	Limit      *int          `form:"limit"` // custom validation in validateAndSetLimit
}

func bindIncidentsQuery(c *gin.Context) (*APIGetIncidentsQuery, error) {
	var query APIGetIncidentsQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		return nil, apiErrors.ErrIncidentFQueryInvalidFormat
	}

	if query.StartDate != nil && query.EndDate != nil && query.EndDate.Before(*query.StartDate) {
		return nil, apiErrors.ErrIncidentFQueryInvalidFormat
	}
	return &query, nil
}

func parseFilterParams(c *gin.Context) (*db.IncidentsParams, error) {
	query, err := bindIncidentsQuery(c)
	if err != nil {
		return nil, err
	}

	params := &db.IncidentsParams{
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
		Impact:    query.Impact,
		IsSystem:  query.System,
	}

	if query.IsActive != nil {
		if !*query.IsActive {
			return nil, apiErrors.ErrIncidentFQueryInvalidFormat
		}
		params.IsActive = query.IsActive
	}

	// Status: Manual validation.
	// validateAndSetStatus there is in validation.go (package v2)
	err = validateAndSetStatus(query.Status, params)
	if err != nil {
		return nil, err
	}

	// parseAndSetComponents check components and set them to db params
	err = parseAndSetComponents(query.Components, params)
	if err != nil {
		return nil, err
	}

	// parseAndSetTypes check event types and set them to db params
	err = parseAndSetTypes(query.Types, params)
	if err != nil {
		return nil, err
	}

	return params, nil
}

func parsePaginationParams(c *gin.Context, params *db.IncidentsParams) error {
	query, err := bindIncidentsQuery(c)
	if err != nil {
		return err
	}

	err = validateAndSetLimit(query.Limit, params)
	if err != nil {
		return err
	}

	page := defaultPageNumber
	if query.Page != nil && *query.Page > 0 {
		page = *query.Page
	}
	params.Page = &page

	limit := defaultIncidentLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	params.Limit = &limit
	return nil
}

func GetIncidentsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve and parse incidents params from query")
		params, err := parseFilterParams(c)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug("retrieve incidents with params", zap.Any("params", params))
		r, err := dbInst.GetEvents(params)
		if err != nil {
			logger.Error("failed to retrieve incidents", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if len(r) == 0 {
			logger.Debug("no incidents found matching the specific criteria", zap.Any("params", params))
			c.JSON(http.StatusOK, gin.H{"data": []Incident{}})
			return
		}

		incidents := make([]*Incident, len(r))
		for i, inc := range r {
			incidents[i] = toAPIEvent(inc)
		}

		c.JSON(http.StatusOK, gin.H{"data": incidents})
	}
}

func GetEventsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve and parse events params from query")
		params, err := parseFilterParams(c)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}
		err = parsePaginationParams(c, params)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug("retrieve events with params", zap.Any("params", params))
		r, total, err := dbInst.GetEventsWithCount(params)
		if err != nil {
			logger.Error("failed to retrieve incidents", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if total == 0 {
			logger.Debug(
				"no incidents found matching the specific criteria",
				zap.Any("params", params),
			)
			c.JSON(http.StatusOK, gin.H{"data": []Incident{}})
			return
		}

		events := make([]*Incident, len(r))
		for i, inc := range r {
			events[i] = toAPIEvent(inc)
		}

		page := 1
		if params.Page != nil {
			page = *params.Page
		}

		limit := defaultIncidentLimit
		if params.Limit != nil {
			limit = *params.Limit
		}

		recordsPerPage := limit
		totalPages := 1
		if limit == 0 {
			recordsPerPage = int(total)
			page = 1
		} else if total > 0 {
			totalPages = int((total + int64(limit) - 1) / int64(limit))
		}

		c.JSON(http.StatusOK, gin.H{
			"data": events,
			"pagination": gin.H{
				"pageIndex":      page,
				"recordsPerPage": recordsPerPage,
				"totalRecords":   total,
				"totalPages":     totalPages,
			},
		})
	}
}

func GetIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve incident")
		var incID IncidentID
		if err := c.ShouldBindUri(&incID); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		r, err := dbInst.GetIncident(incID.ID)
		if err != nil {
			if errors.Is(err, db.ErrDBIncidentDSNotExist) {
				apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrIncidentDSNotExist)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, toAPIEvent(r))
	}
}

func toAPIEvent(inc *db.Incident) *Incident {
	components := make([]int, len(inc.Components))
	for i, comp := range inc.Components {
		components[i] = int(comp.ID)
	}

	updates := mapEventUpdates(inc.Statuses)

	var description string
	if inc.Description != nil {
		description = *inc.Description
	}

	incData := IncidentData{
		Title:       *inc.Text,
		Description: description,
		Impact:      inc.Impact,
		Components:  components,
		StartDate:   *inc.StartDate,
		EndDate:     inc.EndDate,
		System:      &inc.System,
		Updates:     updates,
		Status:      inc.Status,
		Type:        inc.Type,
	}

	return &Incident{IncidentID{ID: int(inc.ID)}, incData}
}

// PostIncidentHandler creates an incident.
func PostIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var incData IncidentData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		incData.StartDate = incData.StartDate.UTC()
		if incData.EndDate != nil {
			*incData.EndDate = incData.EndDate.UTC()
		}

		if err := validateEventCreation(incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		log := logger.With(zap.Any("incidentData", incData))
		log.Info("start to prepare for an incident creation")

		if incData.System == nil {
			var system bool
			incData.System = &system
		}

		var result []*ProcessComponentResp
		var err error
		// Route to appropriate handler based on system field
		if *incData.System {
			log.Info("system incident detected, using system incident creation logic")
			result, err = handleSystemIncidentCreation(dbInst, log, incData)
		} else {
			log.Info("regular incident detected, using regular incident creation logic")
			result, err = handleRegularIncidentCreation(dbInst, log, incData)
		}

		if err != nil {
			if errors.Is(err, apiErrors.ErrIncidentSystemCreationWrongType) {
				apiErrors.RaiseBadRequestErr(c, err)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, PostIncidentResp{Result: result})
	}
}

// handleSystemIncidentCreation handles creation of system incidents.
func handleSystemIncidentCreation(dbInst *db.DB, log *zap.Logger, incData IncidentData) ([]*ProcessComponentResp, error) {

	if incData.Type != event.TypeIncident {
		log.Info("system incident must be of type 'incident'")
		return nil, apiErrors.ErrIncidentSystemCreationWrongType
	}

	if incData.Description == "" {
		// incData.Title = "System Incident from monitoring system"
		incData.Description = "System-wide incident affecting multiple components. Created automatically."
	}

	components := make([]db.Component, len(incData.Components))
	for i, comp := range incData.Components {
		component, err := dbInst.GetComponent(comp)
		if err != nil {
			return nil, err
		}

		components[i] = *component
	}

	var result []*ProcessComponentResp

CompLoop:
	for _, comp := range components {
		log.Info("start to process component", zap.Any("component", comp))
		log.Info("find events with target component", zap.Uint("componentID", comp.ID))
		active := true
		params := &db.IncidentsParams{
			IsActive: &active,
			Types:    []string{event.TypeIncident, event.TypeMaintenance},
		}
		events, err := dbInst.GetEventsByComponentID(comp.ID, params)
		if err != nil {
			return nil, err
		}
		log.Info("found events for component", zap.Uint("componentID", comp.ID), zap.Int("eventsCount", len(events)))

		if len(events) == 0 {
			log.Info("no events found for component, check and process all system incidents", zap.Uint("componentID", comp.ID))
			sysInc, errAdd := addComponentToSystemIncident(dbInst, log, &comp, incData)
			if errAdd != nil {
				return nil, errAdd
			}
			log.Info(
				"component added to system incident",
				zap.Uint("componentID", comp.ID), zap.Uint("incidentID", sysInc.ID),
			)
			result = append(result, &ProcessComponentResp{
				ComponentID: int(comp.ID),
				IncidentID:  int(sysInc.ID),
			})
			continue
		}

		// check existing maintenance events first
		log.Info("checking maintenance for the component", zap.Uint("componentID", comp.ID))
		for _, evnt := range events {
			if evnt.Type == event.TypeMaintenance {
				log.Info("we have maintenance event for the component, skip creation", zap.Uint("eventID", evnt.ID))
				result = append(result, &ProcessComponentResp{
					ComponentID: int(comp.ID),
					IncidentID:  int(evnt.ID),
					Error:       apiErrors.ErrIncidentCreationMaintenanceExists.Error(),
				})
				continue CompLoop
			}

			if !evnt.System {
				log.Info(
					"found non-system incident for the component, return it",
					zap.Uint("componentID", comp.ID), zap.Uint("incidentID", evnt.ID),
				)
				result = append(result, &ProcessComponentResp{
					ComponentID: int(comp.ID),
					IncidentID:  int(evnt.ID),
				})
				continue CompLoop
			}

			log.Info(
				"found system incident for the component, compare impact",
				zap.Uint("componentID", comp.ID), zap.Uint("incidentID", evnt.ID),
			)
			if *evnt.Impact >= *incData.Impact {
				log.Info(
					"existing system incident has equal or higher impact, return the existed incident",
					zap.Uint("componentID", comp.ID), zap.Uint("incidentID", evnt.ID),
				)
				result = append(result, &ProcessComponentResp{
					ComponentID: int(comp.ID),
					IncidentID:  int(evnt.ID),
				})
				continue CompLoop
			}

			// existing system incident has lower impact, move component to the new system incident
			log.Info(
				"found system incident has lower impact, move component to the system incident with the target impact",
				zap.Uint("componentID", comp.ID), zap.Uint("fromIncidentID", evnt.ID),
			)

			sysInc, errAdd := moveComponentFromToSystemIncidents(dbInst, log, &comp, incData, evnt)
			if errAdd != nil {
				return nil, errAdd
			}
			log.Info(
				"component added to system incident",
				zap.Uint("componentID", comp.ID), zap.Uint("incidentID", sysInc.ID),
			)
			result = append(result, &ProcessComponentResp{
				ComponentID: int(comp.ID),
				IncidentID:  int(sysInc.ID),
			})
		}
	}

	return result, nil
}

func addComponentToSystemIncident(
	dbInst *db.DB, log *zap.Logger, comp *db.Component, incData IncidentData,
) (*db.Incident, error) {
	system := true
	active := true

	params := &db.IncidentsParams{
		Types:    []string{event.TypeIncident},
		IsSystem: &system,
		IsActive: &active,
	}
	sysIncidents, errEvents := dbInst.GetEvents(params)
	if errEvents != nil {
		return nil, errEvents
	}

	log.Info("found all system incidents", zap.Int("systemIncidentsCount", len(sysIncidents)))
	// check if we have any system incident with the target impact
	for _, sysInc := range sysIncidents {
		if sysInc.Impact != nil && *sysInc.Impact == *incData.Impact {
			log.Info(
				"found existing system incident with target impact, link component to it",
				zap.Uint("componentID", comp.ID), zap.Uint("incidentID", sysInc.ID),
			)

			sysInc.Components = append(sysInc.Components, *comp)
			sysInc.Statuses = append(sysInc.Statuses, db.IncidentStatus{
				IncidentID: sysInc.ID,
				Status:     sysInc.Status,
				Text:       fmt.Sprintf("%s  added to the incident by system.", comp.PrintAttrs()),
			})
			err := dbInst.ModifyIncident(sysInc)
			if err != nil {
				return nil, err
			}
			return sysInc, nil
		}
	}

	log.Info(
		"no system incident found with the target impact",
		zap.Uint("componentID", comp.ID), zap.Int("impact", *incData.Impact),
	)
	log.Info("creating general system incident with target impact", zap.Int("impact", *incData.Impact))

	incIn := db.Incident{
		Text:        &incData.Title,
		Description: &incData.Description,
		StartDate:   &incData.StartDate,
		EndDate:     incData.EndDate,
		Impact:      incData.Impact,
		System:      *incData.System,
		Type:        incData.Type,
		Components:  []db.Component{*comp},
	}

	if err := createEvent(dbInst, log, &incIn); err != nil {
		return nil, err
	}

	return &incIn, nil
}

func moveComponentFromToSystemIncidents(
	dbInst *db.DB, log *zap.Logger, comp *db.Component, incData IncidentData, oldInc *db.Incident,
) (*db.Incident, error) {
	system := true
	active := true

	params := &db.IncidentsParams{
		Types:    []string{event.TypeIncident},
		IsSystem: &system,
		IsActive: &active,
	}
	sysIncidents, errEvents := dbInst.GetEvents(params)
	if errEvents != nil {
		return nil, errEvents
	}

	log.Info("found all system incidents", zap.Int("systemIncidentsCount", len(sysIncidents)))
	// check if we have any system incident with the target impact
	for _, sysInc := range sysIncidents {
		if sysInc.Impact != nil && *sysInc.Impact == *incData.Impact {
			log.Info(
				"found existing system incident with target impact, move component to it",
				zap.Uint("componentID", comp.ID), zap.Uint("incidentID", sysInc.ID),
			)

			var closeOld bool
			if len(sysInc.Components) == 1 {
				closeOld = true
			}

			inc, err := dbInst.MoveComponentFromOldToAnotherIncident(comp, oldInc, sysInc, closeOld)
			if err != nil {
				return nil, err
			}
			return inc, nil
		}
	}

	log.Info(
		"no system incident found with the target impact",
		zap.Uint("componentID", comp.ID), zap.Int("impact", *incData.Impact),
	)

	if len(oldInc.Components) == 1 {
		log.Info(
			"the source incident has only 1 target component with the lower impact, we can just update its impact",
			zap.Uint("componentID", comp.ID), zap.Uint("incidentID", oldInc.ID),
		)
		inc, err := dbInst.IncreaseIncidentImpact(oldInc, *incData.Impact)
		if err != nil {
			return nil, err
		}
		return inc, nil
	}

	log.Info(
		"the system incident has more components, "+
			"extract the target component to the new system incident with the target impact",
		zap.Int("impact", *incData.Impact),
	)

	inc, err := dbInst.ExtractComponentsToNewIncident(
		[]db.Component{*comp},
		oldInc,
		*incData.Impact,
		incData.Title,
		&incData.Description,
	)
	if err != nil {
		return nil, err
	}

	// Update the new incident to mark it as a system incident
	inc.System = true
	if err = dbInst.ModifyIncident(inc); err != nil {
		return nil, err
	}

	return inc, nil
}

// handleRegularIncidentCreation handles creation of regular incidents with component movement logic.
func handleRegularIncidentCreation(
	dbInst *db.DB, log *zap.Logger, incData IncidentData,
) ([]*ProcessComponentResp, error) {
	components := make([]db.Component, len(incData.Components))
	for i, comp := range incData.Components {
		components[i] = db.Component{ID: uint(comp)}
	}

	incIn := db.Incident{
		Text:        &incData.Title,
		Description: &incData.Description,
		StartDate:   &incData.StartDate,
		EndDate:     incData.EndDate,
		Impact:      incData.Impact,
		System:      *incData.System,
		Type:        incData.Type,
		Components:  components,
	}

	log.Info("get active events from the database")
	isActive := true
	openedIncidents, err := dbInst.GetEvents(&db.IncidentsParams{IsActive: &isActive})
	if err != nil {
		return nil, err
	}

	log.Info("opened incidents and maintenances retrieved", zap.Any("openedIncidents", openedIncidents))

	if err = createEvent(dbInst, log, &incIn); err != nil {
		return nil, err
	}

	// Handle simple cases where no component movement is needed
	if shouldSkipComponentMovement(openedIncidents, incData) {
		return createSimpleIncidentResult(log, &incIn, incData), nil
	}

	// Process component movement for complex cases
	return processComponentMovement(dbInst, log, &incIn, openedIncidents)
}

// shouldSkipComponentMovement determines if component movement logic should be skipped.
func shouldSkipComponentMovement(openedIncidents []*db.Incident, incData IncidentData) bool {
	return len(openedIncidents) == 0 || *incData.Impact == 0 || incData.Type == event.TypeInformation
}

// createSimpleIncidentResult creates a result for incidents that don't require component movement.
func createSimpleIncidentResult(log *zap.Logger, incIn *db.Incident, incData IncidentData) []*ProcessComponentResp {
	if *incData.Impact == 0 {
		log.Info("the event is maintenance or info, finish the incident creation")
	} else {
		log.Info("no opened incidents, finish the incident creation")
	}

	result := make([]*ProcessComponentResp, 0, len(incIn.Components))
	for _, comp := range incIn.Components {
		result = append(result, &ProcessComponentResp{
			ComponentID: int(comp.ID),
			IncidentID:  int(incIn.ID),
		})
	}
	return result
}

// processComponentMovement handles the complex logic of moving components between incidents.
func processComponentMovement(
	dbInst *db.DB, log *zap.Logger, incIn *db.Incident, openedIncidents []*db.Incident,
) ([]*ProcessComponentResp, error) {
	log.Info("start to analyse component movement")
	result := make([]*ProcessComponentResp, 0, len(incIn.Components))

	for _, comp := range incIn.Components {
		compResult, err := processComponentInOpenedIncidents(dbInst, log, &comp, incIn, openedIncidents)
		if err != nil {
			return nil, err
		}
		result = append(result, compResult)
	}

	return result, nil
}

// processComponentInOpenedIncidents processes a single component against all opened incidents.
func processComponentInOpenedIncidents(
	dbInst *db.DB, log *zap.Logger, comp *db.Component, incIn *db.Incident, openedIncidents []*db.Incident,
) (*ProcessComponentResp, error) {
	compResult := &ProcessComponentResp{
		ComponentID: int(comp.ID),
	}

	for _, inc := range openedIncidents {
		if shouldSkipIncident(inc) {
			log.Info(
				"skip the component movement for maintenance or info incident",
				zap.Any("componentID", comp.ID), zap.Any("incident_opened", inc),
			)
			continue
		}

		moved, err := tryMoveComponentIfFound(dbInst, log, comp, inc, incIn, compResult)
		if err != nil {
			return nil, err
		}
		if moved {
			break
		}
	}

	if compResult.IncidentID == 0 {
		log.Info("there are no any opened incidents for given component, return created incident")
		compResult.IncidentID = int(incIn.ID)
	}

	return compResult, nil
}

// shouldSkipIncident determines if an incident should be skipped for component movement.
func shouldSkipIncident(inc *db.Incident) bool {
	return inc.Type == event.TypeInformation || inc.Type == event.TypeMaintenance
}

// tryMoveComponentIfFound attempts to move a component if it's found in the given incident.
func tryMoveComponentIfFound(
	dbInst *db.DB,
	log *zap.Logger,
	comp *db.Component,
	inc *db.Incident,
	incIn *db.Incident,
	compResult *ProcessComponentResp,
) (bool, error) {
	for _, incComp := range inc.Components {
		if comp.ID == incComp.ID {
			log.Info("found the component in the opened incident", zap.Any("component", comp), zap.Any("incident", inc))

			closeInc := len(inc.Components) == 1
			incident, err := dbInst.MoveComponentFromOldToAnotherIncident(comp, inc, incIn, closeInc)
			if err != nil {
				return false, err
			}
			compResult.IncidentID = int(incident.ID)
			return true, nil
		}
	}
	return false, nil
}

type PostIncidentResp struct {
	Result []*ProcessComponentResp `json:"result"`
}

type ProcessComponentResp struct {
	ComponentID int    `json:"component_id"`
	IncidentID  int    `json:"incident_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

func validateEventCreation(incData IncidentData) error {
	if err := validateEventCreationImpact(incData); err != nil {
		return err
	}

	if err := validateEventCreationTimes(incData); err != nil {
		return err
	}

	if len(incData.Updates) != 0 {
		return apiErrors.ErrIncidentUpdatesShouldBeEmpty
	}

	return nil
}

func validateEventCreationImpact(incData IncidentData) error {
	if (incData.Type == event.TypeMaintenance || incData.Type == event.TypeInformation) && *incData.Impact != 0 {
		return apiErrors.ErrIncidentTypeImpactMismatch
	}

	if incData.Type == event.TypeIncident && *incData.Impact == 0 {
		return apiErrors.ErrIncidentTypeImpactMismatch
	}

	return nil
}

func validateEventCreationTimes(incData IncidentData) error {
	// you can't create an incident with the end_date
	if incData.Type == event.TypeIncident && incData.EndDate != nil {
		return apiErrors.ErrIncidentEndDateShouldBeEmpty
	}

	// you can't create an incident in the future
	if incData.Type == event.TypeIncident && incData.StartDate.After(time.Now().UTC()) {
		return apiErrors.ErrIncidentStartDateInFuture
	}

	if incData.Type == event.TypeMaintenance && incData.EndDate == nil {
		return apiErrors.ErrMaintenanceEndDateEmpty
	}

	return nil
}

func createEvent(dbInst *db.DB, log *zap.Logger, inc *db.Incident) error {
	log.Info("start to save an event to the database")
	id, err := dbInst.SaveIncident(inc)
	if err != nil {
		return err
	}

	inc.ID = id

	log.Info("add initial status to the event", zap.Uint("eventID", inc.ID))
	var statusText string
	var status event.Status
	timestamp := time.Now().UTC()
	// Sometimes we have a gap between the start date and the current time.
	// Example: the incident was created now, but we add an update with a detected status since 1-2 seconds.
	// And on the FE it looks like the incident was created in the past.
	// it doesn't affect planned events, like maintenance or info, because they have a start date in the future.
	// However, if someone creates an incident with a start date in the past,
	// we should set up the right timestamp for the status update.
	if inc.StartDate.Before(timestamp) {
		timestamp = *inc.StartDate
	}

	switch inc.Type {
	case event.TypeInformation:
		statusText = event.InfoPlannedStatusText()
		status = event.InfoPlanned
	case event.TypeMaintenance:
		statusText = event.MaintenancePlannedStatusText()
		status = event.MaintenancePlanned
	case event.TypeIncident:
		statusText = event.IncidentDetectedStatusText()
		status = event.IncidentDetected
	}

	inc.Statuses = append(inc.Statuses, db.IncidentStatus{
		IncidentID: inc.ID,
		Status:     status,
		Text:       statusText,
		Timestamp:  timestamp,
	})
	inc.Status = status

	err = dbInst.ModifyIncident(inc)
	if err != nil {
		return err
	}

	return nil
}

type PatchIncidentData struct {
	Title       *string      `json:"title,omitempty"`
	Description *string      `json:"description,omitempty"`
	Impact      *int         `json:"impact,omitempty"`
	Message     string       `json:"message" binding:"required"`
	Status      event.Status `json:"status" binding:"required"`
	UpdateDate  time.Time    `json:"update_date" binding:"required"`
	StartDate   *time.Time   `json:"start_date,omitempty"`
	EndDate     *time.Time   `json:"end_date,omitempty"`
	Type        string       `json:"type,omitempty" binding:"omitempty,oneof=maintenance info incident"`
}

func PatchIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("update incident")

		storedIncident := getEventFromContext(c, logger)

		var incData PatchIncidentData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		incData.UpdateDate = incData.UpdateDate.UTC()
		if incData.StartDate != nil {
			*incData.StartDate = incData.StartDate.UTC()
		}
		if incData.EndDate != nil {
			*incData.EndDate = incData.EndDate.UTC()
		}

		if err := checkPatchData(&incData, storedIncident); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		updateFields(&incData, storedIncident)

		status := db.IncidentStatus{
			IncidentID: storedIncident.ID,
			Status:     incData.Status,
			Text:       incData.Message,
			Timestamp:  incData.UpdateDate,
		}

		storedIncident.Statuses = append(storedIncident.Statuses, status)
		storedIncident.Status = incData.Status

		err := dbInst.ModifyIncident(storedIncident)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if incData.Status == event.IncidentReopened {
			err = dbInst.ReOpenIncident(storedIncident)
			if err != nil {
				apiErrors.RaiseInternalErr(c, err)
				return
			}
		}

		inc, errDB := dbInst.GetIncident(int(storedIncident.ID))
		if errDB != nil {
			apiErrors.RaiseInternalErr(c, errDB)
			return
		}

		c.JSON(http.StatusOK, toAPIEvent(inc))
	}
}

func validateEffectiveTypeAndImpact(effectiveType string, effectiveImpact int) error {
	if (effectiveType == event.TypeMaintenance || effectiveType == event.TypeInformation) && effectiveImpact != 0 {
		return apiErrors.ErrIncidentTypeImpactMismatch
	}
	if effectiveType == event.TypeIncident && effectiveImpact == 0 {
		return apiErrors.ErrIncidentTypeImpactMismatch
	}
	return nil
}

// validateEffectiveTypeAndImpact checks if the incoming type and status are related to each other.
func validateStatusesPatch(incoming *PatchIncidentData, stored *db.Incident) error {
	if stored.Type == event.TypeInformation && !event.IsInformationStatus(incoming.Status) {
		return apiErrors.ErrIncidentPatchInfoStatus
	}

	if stored.Type == event.TypeMaintenance && !event.IsMaintenanceStatus(incoming.Status) {
		return apiErrors.ErrIncidentPatchMaintenanceStatus
	}

	if stored.Type == event.TypeIncident &&
		!event.IsIncidentOpenStatus(incoming.Status) &&
		!event.IsIncidentClosedStatus(incoming.Status) {
		return apiErrors.ErrIncidentPatchIncidentStatus
	}

	return nil
}

func checkPatchData(incoming *PatchIncidentData, stored *db.Incident) error {
	// incoming.Type is now validated by the 'oneof' binding tag in PatchIncidentData
	effectiveType := stored.Type
	if incoming.Type != "" {
		effectiveType = incoming.Type
	}
	effectiveImpact := *stored.Impact
	if incoming.Impact != nil {
		effectiveImpact = *incoming.Impact
	}
	if err := validateEffectiveTypeAndImpact(effectiveType, effectiveImpact); err != nil {
		return err
	}

	if err := validateStatusesPatch(incoming, stored); err != nil {
		return err
	}

	if stored.Type == event.TypeIncident {
		return checkPatchDataForIncident(incoming, stored)
	}

	return nil
}

func checkPatchDataForIncident(incoming *PatchIncidentData, stored *db.Incident) error {
	if stored.EndDate != nil {
		if !event.IsIncidentClosedStatus(incoming.Status) {
			return apiErrors.ErrIncidentPatchClosedStatus
		}

		if (incoming.StartDate != nil || incoming.EndDate != nil) && incoming.Status != event.IncidentChanged {
			return apiErrors.ErrIncidentPatchClosedStatus
		}

		return nil
	}

	if (incoming.Impact != nil && *incoming.Impact != *stored.Impact) &&
		incoming.Status != event.IncidentImpactChanged {
		return apiErrors.ErrIncidentPatchImpactStatusWrong
	}

	if incoming.Impact != nil && *incoming.Impact != *stored.Impact && *incoming.Impact == 0 {
		return apiErrors.ErrIncidentPatchImpactToZeroForbidden
	}

	if incoming.StartDate != nil {
		return apiErrors.ErrIncidentPatchOpenedStartDate
	}

	return nil
}

func updateFields(income *PatchIncidentData, stored *db.Incident) {
	if *stored.Impact == 0 || stored.EndDate != nil {
		if income.StartDate != nil {
			stored.StartDate = income.StartDate
		}

		if income.EndDate != nil {
			stored.EndDate = income.EndDate
		}
	}

	if income.Title != nil {
		stored.Text = income.Title
	}

	if income.Description != nil {
		stored.Description = income.Description
	}

	if income.Impact != nil {
		stored.Impact = income.Impact
	}

	if income.Type != "" {
		stored.Type = income.Type
	}

	stored.Status = income.Status

	if income.Status == event.IncidentReopened {
		stored.EndDate = nil
	}

	if income.Status == event.IncidentResolved {
		stored.EndDate = &income.UpdateDate
	}
}

type PostIncidentSeparateData struct {
	Components []int `json:"components" binding:"required,min=1"`
}

func PostIncidentExtractHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("start to extract components to the new incident")
		storedInc := getEventFromContext(c, logger)

		var incData PostIncidentSeparateData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug(
			"extract components from the incident",
			zap.Any("components", incData.Components),
			zap.Uint("incident_id", storedInc.ID),
		)

		var movedComponents []db.Component
		var movedCounter int
		for _, incCompID := range incData.Components {
			present := false
			for _, storedComp := range storedInc.Components {
				if incCompID == int(storedComp.ID) {
					present = true
					movedComponents = append(movedComponents, storedComp)
					movedCounter++
					break
				}
			}
			if !present {
				apiErrors.RaiseBadRequestErr(c, fmt.Errorf("component %d is not in the incident", incCompID))
				return
			}
		}

		if movedCounter == len(storedInc.Components) {
			apiErrors.RaiseBadRequestErr(c, fmt.Errorf("can not move all components to the new incident, keep at least one"))
			return
		}

		inc, err := dbInst.ExtractComponentsToNewIncident(
			movedComponents,
			storedInc,
			*storedInc.Impact,
			*storedInc.Text,
			storedInc.Description)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, toAPIEvent(inc))
	}
}

type Component struct {
	ComponentID
	Attributes []ComponentAttribute `json:"attributes"`
	Name       string               `json:"name"`
}

type ComponentAvailability struct {
	ComponentID
	Name         string                `json:"name"`
	Availability []MonthlyAvailability `json:"availability"`
	Region       string                `json:"region"`
}

type ComponentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
}

// ComponentAttribute provides additional attributes for component.
// Available list of possible attributes are:
// 1. type
// 2. region
// 3. category
// All of them are required for creation.
type ComponentAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var availableAttrs = map[string]struct{}{ //nolint:gochecknoglobals
	"type":     {},
	"region":   {},
	"category": {},
}

type MonthlyAvailability struct {
	Year       int     `json:"year"`
	Month      int     `json:"month"`      // Number of the month (1 - 12)
	Percentage float64 `json:"percentage"` // Percent (0 - 100 / example: 95.23478)
}

func GetComponentsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve components")

		r, err := dbInst.GetComponentsWithValues()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, r)
	}
}

func GetComponentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve component")

		var compID ComponentID
		if err := c.ShouldBindUri(&compID); err != nil {
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrComponentInvalidFormat)
			return
		}

		r, err := dbInst.GetComponent(compID.ID)
		if err != nil {
			if errors.Is(err, db.ErrDBComponentDSNotExist) {
				apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrComponentDSNotExist)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, r)
	}
}

type PostComponentData struct {
	Attributes []ComponentAttribute `json:"attrs" binding:"required"`
	Name       string               `json:"name" binding:"required"`
}

// PostComponentHandler creates a new component.
func PostComponentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("create a component")

		var component PostComponentData
		if err := c.ShouldBindBodyWithJSON(&component); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		if err := checkComponentAttrs(component.Attributes); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		attrs := make([]db.ComponentAttr, len(component.Attributes))
		for i, attr := range component.Attributes {
			attrs[i] = db.ComponentAttr{
				Name:  attr.Name,
				Value: attr.Value,
			}
		}

		compDB := &db.Component{
			Name:  component.Name,
			Attrs: attrs,
		}

		componentID, err := dbInst.SaveComponent(compDB)
		if err != nil {
			if errors.Is(err, db.ErrDBComponentExists) {
				apiErrors.RaiseBadRequestErr(c, apiErrors.ErrComponentExist)
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusCreated, Component{
			ComponentID: ComponentID{int(componentID)},
			Attributes:  component.Attributes,
			Name:        component.Name,
		})
	}
}

func checkComponentAttrs(attrs []ComponentAttribute) error {
	//nolint:nolintlint,mnd
	// Check total number of attributes
	// this magic number will be changed in the next iteration
	if len(attrs) != 3 {
		return apiErrors.ErrComponentAttrInvalidFormat
	}

	// Track seen attribute names to detect duplicates
	seen := make(map[string]bool)

	// Verify all required attributes exist exactly once
	for _, attr := range attrs {
		if _, exists := availableAttrs[attr.Name]; !exists {
			return apiErrors.ErrComponentAttrInvalidFormat
		}

		// Check for duplicate attributes
		if seen[attr.Name] {
			return apiErrors.ErrComponentAttrInvalidFormat
		}
		seen[attr.Name] = true
	}

	// Verify all required attributes were found
	for requiredAttr := range availableAttrs {
		if !seen[requiredAttr] {
			return apiErrors.ErrComponentAttrInvalidFormat
		}
	}

	return nil
}

func GetComponentsAvailabilityHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve availability of components")

		components, err := dbInst.GetComponentsWithIncidents()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		availability := make([]*ComponentAvailability, len(components))
		for index, comp := range components {
			attrs := make([]ComponentAttribute, len(comp.Attrs))
			for i, attr := range comp.Attrs {
				attrs[i] = ComponentAttribute{
					Name:  attr.Name,
					Value: attr.Value,
				}
			}
			regionValue := ""
			for _, attr := range attrs {
				if attr.Name == "region" {
					regionValue = attr.Value
					break
				}
			}

			incidents := make([]*Incident, len(comp.Incidents))
			for i, inc := range comp.Incidents {
				newInc := &Incident{
					IncidentID: IncidentID{int(inc.ID)},
					IncidentData: IncidentData{
						Title:     *inc.Text,
						Impact:    inc.Impact,
						StartDate: *inc.StartDate,
						EndDate:   inc.EndDate,
						Updates:   nil,
						Type:      inc.Type,
					},
				}
				incidents[i] = newInc
			}

			compAvailability, calcErr := calculateAvailability(&comp)
			if calcErr != nil {
				apiErrors.RaiseInternalErr(c, calcErr)
				return
			}

			sortComponentAvailability(compAvailability)

			availability[index] = &ComponentAvailability{
				ComponentID:  ComponentID{int(comp.ID)},
				Region:       regionValue,
				Name:         comp.Name,
				Availability: compAvailability,
			}
		}

		c.JSON(http.StatusOK, gin.H{"data": availability})
	}
}

func sortComponentAvailability(availabilities []MonthlyAvailability) {
	sort.Slice(availabilities, func(i, j int) bool {
		if availabilities[i].Year == availabilities[j].Year {
			return availabilities[i].Month > availabilities[j].Month
		}
		return availabilities[i].Year > availabilities[j].Year
	})
}

// TODO: add filters for GET request
func calculateAvailability(component *db.Component) ([]MonthlyAvailability, error) {
	const (
		monthsInYear       = 12
		precisionFactor    = 100000
		fullPercentage     = 100
		availabilityMonths = 11
		roundFactor        = 0.5
	)

	if component == nil {
		return nil, fmt.Errorf("component is nil")
	}

	if len(component.Incidents) == 0 {
		return nil, nil
	}

	periodEndDate := time.Now().UTC()
	// Get the current date and starting point (12 months ago)
	// a year ago, including current the month
	periodStartDate := time.Date(periodEndDate.Year(), periodEndDate.Month(),
		1, 0, 0, 0, 0, time.UTC).AddDate(0, -availabilityMonths, 0)
	monthlyDowntime := make([]float64, monthsInYear) // 12 months

	for _, inc := range component.Incidents {
		if inc.EndDate == nil || *inc.Impact != 3 {
			continue
		}

		// here we skip all incidents that are not correspond to our period
		// if the incident started before availability period
		// (as example the incident was started at 01:00 31/12 and finished at 02:00 01/01),
		// we cut the beginning to the period start date, and do the same for the period ending

		incidentStart, incidentEnd, valid := adjustIncidentPeriod(
			*inc.StartDate,
			*inc.EndDate,
			periodStartDate,
			periodEndDate,
		)
		if !valid {
			continue
		}

		current := incidentStart
		for current.Before(incidentEnd) {
			monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
			monthEnd := monthStart.AddDate(0, 1, 0)

			downtimeStart := maxTime(incidentStart, monthStart)
			downtimeEnd := minTime(incidentEnd, monthEnd)
			downtime := downtimeEnd.Sub(downtimeStart).Hours()

			monthIndex := (downtimeStart.Year()-periodStartDate.Year())*monthsInYear +
				int(downtimeStart.Month()-periodStartDate.Month())
			if monthIndex >= 0 && monthIndex < len(monthlyDowntime) {
				monthlyDowntime[monthIndex] += downtime
			}

			current = monthEnd
		}
	}

	monthlyAvailability := make([]MonthlyAvailability, 0, monthsInYear)
	for i := range [monthsInYear]int{} {
		monthDate := periodStartDate.AddDate(0, i, 0)
		totalHours := hoursInMonth(monthDate.Year(), int(monthDate.Month()))
		availability := fullPercentage - (monthlyDowntime[i] / totalHours * fullPercentage)
		availability = float64(int(availability*precisionFactor+roundFactor)) / precisionFactor

		monthlyAvailability = append(monthlyAvailability, MonthlyAvailability{
			Year:       monthDate.Year(),
			Month:      int(monthDate.Month()),
			Percentage: availability,
		})
	}

	return monthlyAvailability, nil
}

// Helper functions for calculateAvailability.
func adjustIncidentPeriod(incidentStart, incidentEnd, periodStart, periodEnd time.Time) (time.Time, time.Time, bool) {
	if incidentEnd.Before(periodStart) || incidentStart.After(periodEnd) {
		return time.Time{}, time.Time{}, false
	}
	if incidentStart.Before(periodStart) {
		incidentStart = periodStart
	}
	if incidentEnd.After(periodEnd) {
		incidentEnd = periodEnd
	}
	return incidentStart, incidentEnd, true
}

func minTime(start, end time.Time) time.Time {
	if start.Before(end) {
		return start
	}
	return end
}

func maxTime(start, end time.Time) time.Time {
	if start.After(end) {
		return start
	}
	return end
}

func hoursInMonth(year int, month int) float64 {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := firstDay.AddDate(0, 1, 0)

	return float64(nextMonth.Sub(firstDay).Hours())
}

type EventUpdateData struct {
	ID        int          `json:"id"`
	Status    event.Status `json:"status"`
	Text      string       `json:"text"`
	Timestamp time.Time    `json:"timestamp"`
}

func bindAndValidatePatchEventUpdate(c *gin.Context) (int, int, string, error) {
	type updateData struct {
		IncidentID int  `uri:"incidentID" binding:"required,gt=0"`
		UpdateID   *int `uri:"updateID" binding:"required,gte=0"`
	}

	type PatchEventUpdateData struct {
		Text string `json:"text" binding:"required"`
	}

	var updData updateData

	if err := c.ShouldBindUri(&updData); err != nil {
		return 0, 0, "", err
	}

	var patchData PatchEventUpdateData
	if err := c.ShouldBindJSON(&patchData); err != nil {
		return 0, 0, "", err
	}
	return updData.IncidentID, *updData.UpdateID, patchData.Text, nil
}

func PatchEventUpdateTextHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug(
			"Patching text for event update",
			zap.String("incidentID", c.Param("incidentID")),
			zap.String("updateID", c.Param("updateID")),
		)

		incID, updID, text, err := bindAndValidatePatchEventUpdate(c)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		// Update existence check.
		updates, err := dbInst.GetEventUpdates(uint(incID))
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if updID < 0 || updID >= len(updates) {
			apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrUpdateDSNotExist)
			return
		}

		targetUPD := updates[updID]
		targetUPD.Text = text

		updated, err := dbInst.ModifyEventUpdate(targetUPD)

		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, EventUpdateData{
			ID:        updID,
			Status:    updated.Status,
			Text:      updated.Text,
			Timestamp: updated.Timestamp,
		})
	}
}

func mapEventUpdates(statuses []db.IncidentStatus) []EventUpdateData {
	updates := make([]EventUpdateData, len(statuses))
	for i, s := range statuses {
		updates[i] = EventUpdateData{
			ID:        i,
			Status:    s.Status,
			Text:      s.Text,
			Timestamp: s.Timestamp,
		}
	}

	return updates
}

func getEventFromContext(c *gin.Context, logger *zap.Logger) *db.Incident {
	val, exists := c.Get("event")
	if !exists {
		logger.Error("event not found in context")
		apiErrors.RaiseInternalErr(c, errors.New("event not found in context"))
		return nil
	}

	evnt, ok := val.(*db.Incident)
	if !ok {
		logger.Error("invalid type in context")
		apiErrors.RaiseInternalErr(c, errors.New("invalid type in context"))
		return nil
	}
	return evnt
}
