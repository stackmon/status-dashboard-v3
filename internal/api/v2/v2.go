package v2

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/common"
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
	StartDate time.Time           `json:"start_date" binding:"required"`
	EndDate   *time.Time          `json:"end_date,omitempty"`
	System    *bool               `json:"system,omitempty"`
	Updates   []db.IncidentStatus `json:"updates,omitempty"`
}

type Incident struct {
	IncidentID
	IncidentData
}

func GetIncidentsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve incidents")
		r, err := dbInst.GetIncidents()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		incidents := make([]*Incident, len(r))
		for i, inc := range r {
			components := make([]int, len(inc.Components))
			for ind, comp := range inc.Components {
				components[ind] = int(comp.ID)
			}

			incidents[i] = &Incident{
				IncidentID: IncidentID{int(inc.ID)},
				IncidentData: IncidentData{
					Title:      *inc.Text,
					Impact:     inc.Impact,
					Components: components,
					StartDate:  *inc.StartDate,
					EndDate:    inc.EndDate,
					System:     &inc.System,
					Updates:    inc.Statuses,
				},
			}
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

		components := make([]int, len(r.Components))
		for i, comp := range r.Components {
			components[i] = int(comp.ID)
		}

		incData := IncidentData{
			Title:      *r.Text,
			Impact:     r.Impact,
			Components: components,
			StartDate:  *r.StartDate,
			EndDate:    r.EndDate,
			System:     &r.System,
			Updates:    r.Statuses,
		}

		c.JSON(http.StatusOK, &Incident{incID, incData})
	}
}

// PostIncidentHandler creates an incident.
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
func PostIncidentHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var incData *IncidentData
		if err := validateIncidentCreation(c, incData); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		log := logger.With(zap.Any("incident", incData))
		log.Info("start to create an incident")

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
		openedIncidents, err := dbInst.GetIncidents(&db.IncidentsParams{IsOpened: true})
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if len(openedIncidents) == 0 {
			log.Info("there are no opened incidents")
			incCreated, errCreation := createIncident(dbInst, log, &incIn, incData.Description)
			if errCreation != nil {
				apiErrors.RaiseInternalErr(c, err)
				return
			}
			c.JSON(http.StatusCreated, incCreated)
			return
		}

		log.Info("start to check incidents for given components", zap.Ints("components", incData.Components))

		//var wg sync.WaitGroup
		//respCh := make(chan *incResp, len(incIn.Components))
		//ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)

		for _, comp := range incIn.Components {
			//wg.Add(1)
			_, _ = processComponent(&comp, &incIn, openedIncidents, log, dbInst, incData.Description)
		}

		//<-respCh
		//wg.Done()

	}
}

type ProcessComponentResp struct {
	ComponentID int
	IncidentID  int
	Error       error
}

func processComponent(comp *db.Component, inc *db.Incident, openedIncidents []*db.Incident, logger *zap.Logger, dbInst *db.DB, desc string) (*db.Incident, error) {
	log := logger.With(zap.Any("component", comp))

	log.Info("find opened incident with the component")

	incident := common.GetIncidentWithComponent(comp.ID, openedIncidents)
	if incident == nil {
		log.Info("there are no incidents with given component, find an incident with incoming impact")
		incByImpact := common.FindIncidentByImpact(*inc.Impact, openedIncidents)
		if incByImpact != nil {
			log.Info(
				"found an incident with given impact, add the component to the incident",
				zap.Intp("impact", incByImpact.Impact),
			)
			incByImpact.Components = append(incByImpact.Components, *comp)
			incByImpact.Statuses = append(incByImpact.Statuses, db.IncidentStatus{
				IncidentID: incByImpact.ID,
				Status:     "SYSTEM",
				Text:       fmt.Sprintf("%s added", comp.PrintAttrs()),
				Timestamp:  time.Now(),
			})
			err := dbInst.ModifyIncident(incByImpact)
			if err != nil {
				return nil, err
			}
			return incByImpact, nil
		}

		log.Info("there are no incidents with given component and impact, create an incident")
		incCreated, errCreation := createIncident(dbInst, log, inc, desc)
		if errCreation != nil {
			return nil, errCreation
		}
		return incCreated, nil
	}

	if *incident.Impact == 0 {
		log.Info("the incident with component in the maintenance status, skip it")
		return nil, apiErrors.ErrIncidentCreationMaintenanceExists
	}
	if *incident.Impact >= *inc.Impact {
		return nil, apiErrors.ErrIncidentCreationLowImpact
	}

	storedIncident, err := common.MoveIncidentToHigherImpact(
		dbInst, log, comp,
		incident, openedIncidents,
		*inc.Impact, *inc.Text)
	if err != nil {
		return nil, err
	}

	return storedIncident, nil
}

func validateIncidentCreation(c *gin.Context, incData *IncidentData) error {
	if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
		return err
	}

	if *incData.Impact != 0 && incData.EndDate != nil {
		return apiErrors.ErrIncidentEndDateShouldBeEmpty
	}

	if len(incData.Updates) != 0 {
		return apiErrors.ErrIncidentUpdatesShouldBeEmpty
	}

	return nil
}

func createIncident(dbInst *db.DB, log *zap.Logger, inc *db.Incident, description string) (*db.Incident, error) {
	log.Info("start to create an incident")
	id, err := dbInst.SaveIncident(inc)
	if err != nil {
		return nil, err
	}

	inc.ID = id

	if *inc.Impact == 0 && description != "" {
		log.Info("the incident is maintenance for component, add description")

		inc.Statuses = append(inc.Statuses, db.IncidentStatus{
			IncidentID: inc.ID,
			// TODO: add another status for this action, legacy
			Status:    "description",
			Text:      description,
			Timestamp: time.Now(),
		})

		err = dbInst.ModifyIncident(inc)
		if err != nil {
			return nil, err
		}
	}

	return inc, nil
}

type PatchIncidentData struct {
	Title *string `json:"title,omitempty"`
	//    INCIDENT_IMPACTS = {
	//        0: Impact(0, "maintenance", "Scheduled maintenance"),
	//        1: Impact(1, "minor", "Minor incident (i.e. performance impact)"),
	//        2: Impact(2, "major", "Major incident"),
	//        3: Impact(3, "outage", "Service outage"),
	//    }
	Impact     *int  `json:"impact,omitempty"`
	Components []int `json:"components,omitempty"`
	// Datetime format is standard: "2006-01-01T12:00:00Z"
	StartDate *time.Time         `json:"start_date,omitempty"`
	EndDate   *time.Time         `json:"end_date,omitempty"`
	System    *bool              `json:"system,omitempty"`
	Update    *db.IncidentStatus `json:"update,omitempty"`
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

		var components []db.Component
		if len(incData.Components) != 0 {
			components = make([]db.Component, len(incData.Components))
			for i, comp := range incData.Components {
				components[i] = db.Component{ID: uint(comp)}
			}
		}

		var statuses []db.IncidentStatus
		if incData.Update != nil {
			statuses = append(statuses, *incData.Update)
		}

		dbInc := db.Incident{
			ID:         uint(incID.ID),
			Text:       incData.Title,
			StartDate:  incData.StartDate,
			EndDate:    incData.EndDate,
			Impact:     incData.Impact,
			System:     *incData.System,
			Components: components,
			Statuses:   statuses,
		}

		err := dbInst.ModifyIncident(&dbInc)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "incident updated"})
	}
}

type Component struct {
	ComponentID
	Attributes []ComponentAttribute `json:"attributes"`
	Name       string               `json:"name"`
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
	// this magic number will be changed in the next iteration
	if len(attrs) != 3 {
		return apiErrors.ErrComponentAttrInvalidFormat
	}
	for _, attr := range attrs {
		_, ok := availableAttrs[attr.Name]
		if !ok {
			return apiErrors.ErrComponentAttrInvalidFormat
		}
	}

	return nil
}
