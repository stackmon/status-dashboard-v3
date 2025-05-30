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
)

type IncidentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
}

type IncidentData struct {
	Title string `json:"title" binding:"required"`
	//TODO: this field only valid for incident creation (legacy), but it should be an additional field in DB.
	Description string `json:"description,omitempty"`
	//    INCIDENT_IMPACTS = {
	//        0: Impact(0, "maintenance", "Scheduled maintenance"),
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
	//    2. incident
	// TODO: "Type" field should be mandatory after the migration
	Type    string              `json:"type,omitempty"`
	Updates []db.IncidentStatus `json:"updates,omitempty"`
}

type Incident struct {
	IncidentID
	IncidentData
}

type APIGetIncidentsQuery struct {
	Type       *string    `form:"type" binding:"omitempty,oneof=maintenance incident"`
	Opened     *bool      `form:"opened" binding:"omitempty"`
	Status     *string    `form:"status"` // Custom validation in validateAndSetStatus
	StartDate  *time.Time `form:"start_date" binding:"omitempty"`
	EndDate    *time.Time `form:"end_date" binding:"omitempty"`
	Impact     *int       `form:"impact" binding:"omitempty,gte=0,lte=3"`
	System     *bool      `form:"system" binding:"omitempty"`
	Components *string    `form:"components"` // Custom validation in parseAndSetComponents
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

func parseIncidentParams(c *gin.Context) (*db.IncidentsParams, error) {
	query, err := bindIncidentsQuery(c)
	if err != nil {
		return nil, err
	}

	params := &db.IncidentsParams{}

	// Type: Validated by "oneof" tag in APIGetIncidentsQuery.
	if query.Type != nil {
		params.Type = query.Type
	}

	// Opened: Type is now *bool.
	// Gin handles parsing "true", "false", "1", "0".
	if query.Opened != nil {
		params.IsOpened = query.Opened
	}

	// Status: Manual validation.
	// validateAndSetStatus there is in validation.go (package v2)
	err = validateAndSetStatus(query.Status, params)
	if err != nil {
		return nil, err
	}

	// Dates: Parsed by time_format tag.
	// Range validated in bindAndValidateIncidentsQuery.
	if query.StartDate != nil {
		params.StartDate = query.StartDate
	}
	if query.EndDate != nil {
		params.EndDate = query.EndDate
	}

	// Impact: Type *int, validated by gte/lte tags.
	if query.Impact != nil {
		params.Impact = query.Impact
	}

	if query.System != nil {
		params.IsSystem = query.System
	}

	// parseAndSetComponents there is in validation.go (package v2)
	err = parseAndSetComponents(query.Components, params)
	if err != nil {
		return nil, err
	}
	return params, nil
}

// GetIncidentsHandler retrieves incidents based on query parameters.
func GetIncidentsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("parsing incidents params")

		params, err := parseIncidentParams(c)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug("retrieve incidents with filters", zap.Any("params", params))
		r, err := dbInst.GetIncidents(params)
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
			incidents[i] = toAPIIncident(inc)
		}

		c.JSON(http.StatusOK, gin.H{"data": incidents})
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

		c.JSON(http.StatusOK, toAPIIncident(r))
	}
}

func toAPIIncident(inc *db.Incident) *Incident {
	components := make([]int, len(inc.Components))
	for i, comp := range inc.Components {
		components[i] = int(comp.ID)
	}

	incData := IncidentData{
		Title:      *inc.Text,
		Impact:     inc.Impact,
		Components: components,
		StartDate:  *inc.StartDate,
		EndDate:    inc.EndDate,
		System:     &inc.System,
		Updates:    inc.Statuses,
	}
	if *inc.Impact == 0 {
		incData.Type = "maintenance"
	} else {
		incData.Type = "incident"
	}

	return &Incident{IncidentID{ID: int(inc.ID)}, incData}
}

// TODO: copy-paste from the legacy, it's implemented, but only for API. We should discuss about this functionality.
//
//	 Process component status update and open new incident if required:
//
//	- current active maintenance for the component - do nothing
//	- current active incident for the component - do nothing
//	- current active incident NOT for the component - add component into
//	  the list of affected components
//	- no active incidents - create new one
//	- current active incident for the component and requested
//	  impact > current impact - run handling:
//
//	  If a component exists in an incident, but the requested
//	  impact is higher than the current one, then the component
//	  will be moved to another incident if it exists with the
//	  requested impact, otherwise a new incident will be created
//	  and the component will be moved to the new incident.
//	  If there is only one component in an incident, and an
//	  incident with the requested impact does not exist,
//	  then the impact of the incident will be changed to a higher
//	  one, otherwise the component will be moved to an existing
//	  incident with the requested impact, and the current incident
//	  will be closed by the system.
//	  The movement of a component and the closure of an incident
//	  will be reflected in the incident statuses.
//
// TODO: skip this check, will be redesigned after the new incident management
// PostIncidentHandler creates an incident.
func PostIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc { //nolint:gocognit
	return func(c *gin.Context) {
		var incData IncidentData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		if err := validateIncidentCreation(incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		log := logger.With(zap.Any("incident", incData))
		log.Info("start to prepare for an incident creation")

		components := make([]db.Component, len(incData.Components))
		for i, comp := range incData.Components {
			components[i] = db.Component{ID: uint(comp)}
		}

		if incData.System == nil {
			var system bool
			incData.System = &system
		}

		incIn := db.Incident{
			Text:       &incData.Title,
			StartDate:  &incData.StartDate,
			EndDate:    incData.EndDate,
			Impact:     incData.Impact,
			System:     *incData.System,
			Components: components,
		}

		log.Info("get opened incidents")
		isOpenedTrue := true
		openedIncidents, err := dbInst.GetIncidents(&db.IncidentsParams{IsOpened: &isOpenedTrue})
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if err = createIncident(dbInst, log, &incIn, incData.Description); err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if len(openedIncidents) == 0 || *incData.Impact == 0 {
			if *incData.Impact == 0 {
				log.Info("the incident is maintenance, finish the incident creation")
			} else {
				log.Info("no opened incidents, finish the incident creation")
			}
			result := make([]*ProcessComponentResp, len(incIn.Components))
			for i, comp := range incIn.Components {
				result[i] = &ProcessComponentResp{
					ComponentID: int(comp.ID),
					IncidentID:  int(incIn.ID),
				}
			}

			c.JSON(http.StatusOK, PostIncidentResp{Result: result})
			return
		}

		log.Info("start to analyse component movement")
		result := make([]*ProcessComponentResp, 0)
		// holly shit, but it moved from original logic
		for _, comp := range incIn.Components {
			compResult := &ProcessComponentResp{
				ComponentID: int(comp.ID),
			}
			for _, inc := range openedIncidents {
				for _, incComp := range inc.Components {
					if comp.ID == incComp.ID {
						log.Info("found the component in the opened incident", zap.Any("component", comp), zap.Any("incident", inc))
						var closeInc bool
						if len(inc.Components) == 1 {
							closeInc = true
						}
						incident, errRes := dbInst.MoveComponentFromOldToAnotherIncident(&comp, inc, &incIn, closeInc)
						if errRes != nil {
							apiErrors.RaiseInternalErr(c, err)
							return
						}
						compResult.IncidentID = int(incident.ID)
					}
				}
			}
			if compResult.IncidentID == 0 {
				log.Info("there are no any opened incidents for given component, return created incident")
				compResult.IncidentID = int(incIn.ID)
			}
			result = append(result, compResult)
		}

		c.JSON(http.StatusOK, PostIncidentResp{Result: result})
	}
}

type PostIncidentResp struct {
	Result []*ProcessComponentResp `json:"result"`
}

type ProcessComponentResp struct {
	ComponentID int    `json:"component_id"`
	IncidentID  int    `json:"incident_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

func validateIncidentCreation(incData IncidentData) error {
	if *incData.Impact != 0 && incData.EndDate != nil {
		return apiErrors.ErrIncidentEndDateShouldBeEmpty
	}

	if len(incData.Updates) != 0 {
		return apiErrors.ErrIncidentUpdatesShouldBeEmpty
	}

	return nil
}

func createIncident(dbInst *db.DB, log *zap.Logger, inc *db.Incident, description string) error {
	log.Info("start to create an incident")
	id, err := dbInst.SaveIncident(inc)
	if err != nil {
		return err
	}

	inc.ID = id

	if *inc.Impact == 0 && description != "" {
		log.Info("the incident is maintenance for component, add description")

		inc.Statuses = append(inc.Statuses, db.IncidentStatus{
			IncidentID: inc.ID,
			// TODO: add another status for this action, legacy
			Status:    "description",
			Text:      description,
			Timestamp: time.Now().UTC(),
		})

		err = dbInst.ModifyIncident(inc)
		if err != nil {
			return err
		}
	}

	return nil
}

type PatchIncidentData struct {
	Title      *string    `json:"title,omitempty"`
	Impact     *int       `json:"impact,omitempty"`
	Message    string     `json:"message" binding:"required"`
	Status     string     `json:"status" binding:"required"`
	UpdateDate time.Time  `json:"update_date" binding:"required"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}

func PatchIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("update incident")

		var incID IncidentID
		if err := c.ShouldBindUri(&incID); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		var incData PatchIncidentData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		storedIncident, err := dbInst.GetIncident(incID.ID)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if err = checkPatchData(&incData, storedIncident); err != nil {
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

		err = dbInst.ModifyIncident(storedIncident)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if incData.Status == IncidentReopened {
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

		c.JSON(http.StatusOK, toAPIIncident(inc))
	}
}

func checkPatchData(incoming *PatchIncidentData, stored *db.Incident) error {
	if *stored.Impact == 0 {
		if incoming.Impact != nil && *incoming.Impact != 0 {
			return apiErrors.ErrIncidentPatchMaintenanceImpactForbidden
		}

		if _, ok := maintenanceStatuses[incoming.Status]; !ok {
			return apiErrors.ErrIncidentPatchMaintenanceStatus
		}

		return nil
	}

	if stored.EndDate != nil {
		if _, ok := incidentClosedStatuses[incoming.Status]; !ok {
			return apiErrors.ErrIncidentPatchClosedStatus
		}

		if (incoming.StartDate != nil || incoming.EndDate != nil) && incoming.Status != IncidentChanged {
			return apiErrors.ErrIncidentPatchClosedStatus
		}

		return nil
	}

	if (incoming.Impact != nil && *incoming.Impact != *stored.Impact) && incoming.Status != IncidentImpactChanged {
		return apiErrors.ErrIncidentPatchImpactStatusWrong
	}

	if incoming.Impact != nil && *incoming.Impact != *stored.Impact && *incoming.Impact == 0 {
		return apiErrors.ErrIncidentPatchImpactToMaintenanceForbidden
	}

	if _, ok := incidentOpenStatuses[incoming.Status]; !ok {
		return apiErrors.ErrIncidentPatchStatus
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

	if income.Impact != nil {
		stored.Impact = income.Impact
	}

	if income.Status == IncidentReopened {
		stored.EndDate = nil
	}

	if income.Status == IncidentResolved {
		stored.EndDate = &income.UpdateDate
	}
}

type PostIncidentSeparateData struct {
	Components []int `json:"components" binding:"required,min=1"`
}

func PostIncidentExtractHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc { //nolint:gocognit
	return func(c *gin.Context) {
		logger.Debug("start to extract components to the new incident")

		var incID IncidentID
		if err := c.ShouldBindUri(&incID); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		var incData PostIncidentSeparateData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug(
			"extract components from the incident",
			zap.Any("components", incData.Components),
			zap.Int("incident_id", incID.ID),
		)

		storedInc, err := dbInst.GetIncident(incID.ID)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

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

		inc, err := dbInst.ExtractComponentsToNewIncident(movedComponents, storedInc, *storedInc.Impact, *storedInc.Text)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, toAPIIncident(inc))
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

	periodEndDate := time.Now()
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
