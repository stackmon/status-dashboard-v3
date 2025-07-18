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

type IncidentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
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
	Type    string              `json:"type" binding:"required,oneof=maintenance info incident"`
	Updates []db.IncidentStatus `json:"updates,omitempty"`
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

// GetIncidentsHandler retrieves incidents based on query parameters.
func GetIncidentsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve and parse incidents params from query")

		params, err := parseIncidentParams(c)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		logger.Debug("retrieve incidents with params", zap.Any("params", params))
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
		Updates:     inc.Statuses,
		Type:        inc.Type,
	}

	return &Incident{IncidentID{ID: int(inc.ID)}, incData}
}

// PostIncidentHandler creates an incident.
func PostIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc { //nolint:gocognit,funlen
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

		components := make([]db.Component, len(incData.Components))
		for i, comp := range incData.Components {
			components[i] = db.Component{ID: uint(comp)}
		}

		if incData.System == nil {
			var system bool
			incData.System = &system
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
		openedIncidents, err := dbInst.GetIncidents(&db.IncidentsParams{IsActive: &isActive})
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		log.Info("opened incidents and maintenances retrieved", zap.Any("openedIncidents", openedIncidents))

		if err = createEvent(dbInst, log, &incIn); err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if len(openedIncidents) == 0 || *incData.Impact == 0 || incData.Type == event.TypeInformation {
			if *incData.Impact == 0 {
				log.Info("the event is maintenance or info, finish the incident creation")
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
		// It moved from original logic
		for _, comp := range incIn.Components {
			compResult := &ProcessComponentResp{
				ComponentID: int(comp.ID),
			}
			for _, inc := range openedIncidents {
				if inc.Type == event.TypeInformation || inc.Type == event.TypeMaintenance {
					log.Info(
						"skip the component movement for maintenance or info incident",
						zap.Any("componentID", comp.ID), zap.Any("incident_opened", inc),
					)
					continue
				}
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
		Timestamp:  time.Now().UTC(),
	})

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

func PatchIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc { //nolint:gocognit
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

		// Ensure the update date is in UTC
		incData.UpdateDate = incData.UpdateDate.UTC()
		if incData.StartDate != nil {
			*incData.StartDate = incData.StartDate.UTC()
		}
		if incData.EndDate != nil {
			*incData.EndDate = incData.EndDate.UTC()
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

		c.JSON(http.StatusOK, toAPIIncident(inc))
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

func validateMaintenancePatch(incoming *PatchIncidentData) error {
	if !event.IsMaintenanceStatus(incoming.Status) {
		return apiErrors.ErrIncidentPatchMaintenanceStatus
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

	if *stored.Impact == 0 {
		return validateMaintenancePatch(incoming)
	}

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
		return apiErrors.ErrIncidentPatchImpactToMaintenanceForbidden
	}

	if !event.IsIncidentOpenStatus(incoming.Status) {
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

	if income.Description != nil {
		stored.Description = income.Description
	}

	if income.Impact != nil {
		stored.Impact = income.Impact
	}

	if income.Type != "" {
		stored.Type = income.Type
	}

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
