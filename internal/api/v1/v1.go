package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const (
	timeLayout = "2006-01-02 15:04"
)

type IncidentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
}

type IncidentData struct {
	Text string `json:"text" binding:"required"`
	//    INCIDENT_IMPACTS = {
	//        0: Impact(0, "maintenance", "Scheduled maintenance"),
	//        1: Impact(1, "minor", "Minor incident (i.e. performance impact)"),
	//        2: Impact(2, "major", "Major incident"),
	//        3: Impact(3, "outage", "Service outage"),
	//    }
	Impact *int `json:"impact" binding:"required,gte=0,lte=3"`
	// datetime format is "2006-01-01 12:00"
	StartDate SD2Time           `json:"start_date" binding:"required"`
	EndDate   *SD2Time          `json:"end_date"`
	Updates   []*IncidentStatus `json:"updates"`
}

// IncidentStatus is a db table representation.
type IncidentStatus struct {
	Status    string  `json:"status"`
	Text      string  `json:"text"`
	Timestamp SD2Time `json:"timestamp"`
}

type Incident struct {
	IncidentID
	IncidentData
}

type SD2Time time.Time

func (s *SD2Time) MarshalJSON() ([]byte, error) {
	sTime := time.Time(*s)
	str := sTime.Format(timeLayout)

	return json.Marshal(str)
}

func (s *SD2Time) UnmarshalJSON(data []byte) error {
	strData := string(data)
	if strData == "null" {
		*s = SD2Time{}
		return nil
	}
	if strings.HasPrefix(strData, "\"") {
		runes := []rune(strData)
		strData = string(runes[1 : len(runes)-1])
	}
	t, err := time.Parse(timeLayout, strData)
	if err != nil {
		return err
	}
	*s = SD2Time(t)
	return nil
}

func GetIncidentsHandler(db *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve incidents")
		r, err := db.GetIncidents()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		incidents := make([]*Incident, len(r))
		for i, inc := range r {
			updates := make([]*IncidentStatus, len(inc.Statuses))
			for index, status := range inc.Statuses {
				updates[index] = &IncidentStatus{
					Status:    status.Status,
					Text:      status.Text,
					Timestamp: SD2Time(status.Timestamp),
				}
			}

			var endDate *SD2Time
			if inc.EndDate != nil {
				sd2T := SD2Time(*inc.EndDate)
				endDate = &sd2T
			}

			incidents[i] = &Incident{
				IncidentID: IncidentID{int(inc.ID)},
				IncidentData: IncidentData{
					Text:      *inc.Text,
					Impact:    inc.Impact,
					StartDate: SD2Time(*inc.StartDate),
					EndDate:   endDate,
					Updates:   updates,
				},
			}
		}

		c.JSON(http.StatusOK, incidents)
	}
}

type Component struct {
	ComponentID
	Attrs     []*ComponentAttribute `json:"attributes"`
	Name      string                `json:"name"`
	Incidents []*Incident           `json:"incidents"`
}

type ComponentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
}

type ComponentAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func GetComponentsStatusHandler(db *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("retrieve components with incidents")
		r, err := db.GetComponentsWithIncidents()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		// We can't change this logic, because of date representation.
		// Will be changed in the V2.
		components := make([]*Component, len(r))
		for index, component := range r {
			attrs := make([]*ComponentAttribute, len(component.Attrs))
			for i, attr := range component.Attrs {
				attrs[i] = &ComponentAttribute{
					Name:  attr.Name,
					Value: attr.Value,
				}
			}

			incidents := make([]*Incident, len(component.Incidents))
			for i, inc := range component.Incidents {
				var endDate *SD2Time
				if inc.EndDate != nil {
					sd2T := SD2Time(*inc.EndDate)
					endDate = &sd2T
				}

				newInc := &Incident{
					IncidentID: IncidentID{int(inc.ID)},
					IncidentData: IncidentData{
						Text:      *inc.Text,
						Impact:    inc.Impact,
						StartDate: SD2Time(*inc.StartDate),
						EndDate:   endDate,
						Updates:   nil,
					},
				}

				updates := make([]*IncidentStatus, len(inc.Statuses))
				for ind, status := range inc.Statuses {
					updates[ind] = &IncidentStatus{
						Status:    status.Status,
						Text:      status.Text,
						Timestamp: SD2Time(status.Timestamp),
					}
				}

				newInc.Updates = updates

				incidents[i] = newInc
			}

			components[index] = &Component{
				ComponentID: ComponentID{int(component.ID)},
				Attrs:       attrs,
				Name:        component.Name,
				Incidents:   incidents,
			}
		}

		c.JSON(http.StatusOK, components)
	}
}

type ComponentStatusPost struct {
	Name       string                `json:"name" binding:"required"`
	Impact     int                   `json:"impact" binding:"required,gte=1,lte=3"`
	Text       string                `json:"text"`
	Attributes []*ComponentAttribute `json:"attributes" binding:"required"`
}

// PostComponentStatusHandler creates a new component.
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
func PostComponentStatusHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc { //nolint:gocognit
	return func(c *gin.Context) {
		var inComponent ComponentStatusPost
		attr, err := extractComponentAttr(c, &inComponent)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		log := logger.With(zap.Any("component", inComponent))

		log.Info("get component from name and attributes")
		storedComponent, err := dbInst.GetComponentFromNameAttrs(inComponent.Name, attr)
		if err != nil {
			if errors.Is(err, db.ErrDBComponentDSNotExist) {
				apiErrors.RaiseBadRequestErr(c, apiErrors.ErrComponentDSNotExist)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		log.Info("get opened incidents")
		openedIncidents, err := dbInst.GetIncidents(&db.IncidentsParams{IsOpened: true})
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if len(openedIncidents) == 0 {
			log.Info("there are no opened incidents")
			inc, errCreation := createIncident(dbInst, log, storedComponent, &inComponent)
			if errCreation != nil {
				apiErrors.RaiseInternalErr(c, err)
				return
			}
			c.JSON(http.StatusCreated, inc)
			return
		}

		log.Info("find opened incident with the component")
		// the strange logic, because we will get the first incident with component, but we can have another one
		incident := getIncidentWithComponent(storedComponent.ID, openedIncidents)
		if incident == nil {
			log.Info("there are no incidents with given component, find an incident with incoming impact")
			incByImpact := findIncidentByImpact(inComponent.Impact, openedIncidents)
			if incByImpact != nil {
				log.Info(
					"found an incident with given impact, add the component to the incident",
					zap.Intp("impact", incByImpact.Impact),
				)
				incByImpact.Components = append(incByImpact.Components, *storedComponent)
				incByImpact.Statuses = append(incByImpact.Statuses, db.IncidentStatus{
					IncidentID: incByImpact.ID,
					Status:     "SYSTEM",
					Text:       fmt.Sprintf("%s added", storedComponent.PrintAttrs()),
					Timestamp:  time.Now(),
				})
				err = dbInst.ModifyIncident(incByImpact)
				if err != nil {
					apiErrors.RaiseInternalErr(c, err)
					return
				}
				c.JSON(http.StatusCreated, toAPIIncident(incByImpact))
				return
			}

			log.Info("there are no incidents with given component and impact, create an incident")
			inc, errCreation := createIncident(dbInst, log, storedComponent, &inComponent)
			if errCreation != nil {
				apiErrors.RaiseInternalErr(c, err)
				return
			}
			c.JSON(http.StatusCreated, inc)
			return
		}

		if *incident.Impact == 0 {
			log.Info("the incident with component in the maintenance status, skip it")
			// the status code is the legacy
			c.JSON(http.StatusCreated, toAPIIncident(incident))
			return
		}

		if *incident.Impact >= inComponent.Impact {
			log.Info("the incident impact is higher than incoming, skip it")
			c.JSON(http.StatusConflict, returnConflictResponse(storedComponent, incident))
			return
		}

		storedIncident, err := moveIncidentToHigherImpact(
			dbInst, log, storedComponent,
			incident, openedIncidents,
			inComponent.Impact, inComponent.Text)
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.JSON(http.StatusCreated, toAPIIncident(storedIncident))
	}
}

func extractComponentAttr(c *gin.Context, in *ComponentStatusPost) (*db.ComponentAttr, error) {
	if err := c.ShouldBindBodyWithJSON(in); err != nil {
		return nil, apiErrors.ErrComponentInvalidFormat
	}

	var dbAttr *db.ComponentAttr
	for _, attr := range in.Attributes {
		if attr.Name == "region" {
			dbAttr = &db.ComponentAttr{
				Name:  attr.Name,
				Value: attr.Value,
			}
		}
	}

	if dbAttr == nil {
		return nil, apiErrors.ErrComponentRegionAttrMissing
	}

	return dbAttr, nil
}

type ConflictResponse struct {
	Msg                   string        `json:"message"`
	TargetComponent       *db.Component `json:"targetComponent"`
	ExistingIncidentID    int           `json:"existingIncidentId"`
	ExistingIncidentTitle string        `json:"existingIncidentTitle"`
	Details               string        `json:"details"`
}

const (
	conflictMsg     = "Incident with this the component already exists"
	conflictDetails = "Check your request parameters"
)

func returnConflictResponse(comp *db.Component, inc *db.Incident) *ConflictResponse {
	return &ConflictResponse{
		Msg:                   conflictMsg,
		TargetComponent:       comp,
		ExistingIncidentID:    int(inc.ID),
		ExistingIncidentTitle: *inc.Text,
		Details:               conflictDetails,
	}
}

func createIncident(
	dbInst *db.DB, log *zap.Logger, storedComponent *db.Component, inComponent *ComponentStatusPost,
) (*Incident, error) {
	log.Info("start to create an incident")
	startDate := time.Now()
	comps := []db.Component{*storedComponent}
	inc := &db.Incident{
		Text:       &inComponent.Text,
		StartDate:  &startDate,
		EndDate:    nil,
		Impact:     &inComponent.Impact,
		Statuses:   nil,
		Components: comps,
	}
	id, err := dbInst.SaveIncident(inc)
	if err != nil {
		return nil, err
	}
	inc.ID = id
	return toAPIIncident(inc), nil
}

func getIncidentWithComponent(componentID uint, incidents []*db.Incident) *db.Incident {
	for i, incident := range incidents {
		for _, component := range incident.Components {
			if componentID == component.ID {
				return incidents[i]
			}
		}
	}

	return nil
}

func toAPIIncident(incident *db.Incident) *Incident {
	updates := make([]*IncidentStatus, len(incident.Statuses))
	for i, s := range incident.Statuses {
		updates[i] = &IncidentStatus{
			Status:    s.Status,
			Text:      s.Text,
			Timestamp: SD2Time(s.Timestamp),
		}
	}
	return &Incident{
		IncidentID: IncidentID{ID: int(incident.ID)},
		IncidentData: IncidentData{
			Text:      *incident.Text,
			Impact:    incident.Impact,
			StartDate: SD2Time(*incident.StartDate),
			Updates:   updates,
		},
	}
}

func moveIncidentToHigherImpact(
	dbInst *db.DB, log *zap.Logger,
	storedComponent *db.Component, incident *db.Incident, incidents []*db.Incident,
	impact int, text string,
) (*db.Incident, error) {
	incWithHighImpact := findIncidentByImpact(impact, incidents)
	if incWithHighImpact == nil {
		if len(incident.Components) > 1 {
			log.Info("no active incidents with requested impact, opening the new one")
			return dbInst.ExtractComponentToNewIncident(storedComponent, incident, impact, text)
		}
		log.Info(
			"only one component in the incident, increase impact",
			zap.Intp("oldImpact", incident.Impact),
			zap.Int("newImpact", impact),
		)
		return dbInst.IncreaseIncidentImpact(incident, impact)
	}

	if len(incident.Components) == 1 {
		log.Info("move component to the incident with the found impact, close current incident")
		return dbInst.MoveComponentFromOldToAnotherIncident(storedComponent, incident, incWithHighImpact, true)
	}

	// In that case we have the existed incident with target impact (greater where component is presented)
	// And count of components is more than one. We should move component from old to new.
	log.Info("move component to the incident with the higher impact")
	return dbInst.MoveComponentFromOldToAnotherIncident(storedComponent, incident, incWithHighImpact, false)
}

func findIncidentByImpact(impact int, incidents []*db.Incident) *db.Incident {
	for _, incident := range incidents {
		if *incident.Impact == impact {
			return incident
		}
	}
	return nil
}
