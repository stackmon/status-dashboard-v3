package app

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

type IncidentID struct {
	ID int `json:"id" uri:"id" binding:"required,gte=0"`
}

type IncidentData struct {
	Title string `json:"title" binding:"required"`
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
	System    bool                `json:"system,omitempty"`
	Updates   []db.IncidentStatus `json:"updates,omitempty"`
}

type Incident struct {
	IncidentID
	IncidentData
}

func (a *App) GetIncidentsHandler(c *gin.Context) {
	r, err := a.DB.GetIncidents()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint:nolintlint,errcheck
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
				System:     *inc.System,
				Updates:    inc.Statuses,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": incidents})
}

func (a *App) GetIncidentHandler(c *gin.Context) {
	var incID IncidentID
	if err := c.ShouldBindUri(&incID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint:nolintlint,errcheck
		return
	}

	r, err := a.DB.GetIncident(incID.ID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint:nolintlint,errcheck
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
		System:     *r.System,
		Updates:    r.Statuses,
	}

	c.JSON(http.StatusOK, &Incident{incID, incData})
}

func (a *App) PostIncidentHandler(c *gin.Context) {
	var incData IncidentData
	if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint:nolintlint,errcheck
		return
	}

	components := make([]db.Component, len(incData.Components))
	for i, comp := range incData.Components {
		components[i] = db.Component{ID: uint(comp)}
	}

	dbInc := db.Incident{
		Text:       &incData.Title,
		StartDate:  &incData.StartDate,
		EndDate:    incData.EndDate,
		Impact:     incData.Impact,
		System:     &incData.System,
		Components: components,
	}

	incidentID, err := a.DB.SaveIncident(&dbInc)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint:nolintlint,errcheck
		return
	}

	c.JSON(http.StatusOK, Incident{
		IncidentID:   IncidentID{int(incidentID)},
		IncidentData: incData,
	})
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

func (a *App) PatchIncidentHandler(c *gin.Context) {
	var incID IncidentID
	if err := c.ShouldBindUri(&incID); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint:nolintlint,errcheck
	}

	var incData PatchIncidentData
	if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint:nolintlint,errcheck
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
		System:     incData.System,
		Components: components,
		Statuses:   statuses,
	}

	err := a.DB.ModifyIncident(&dbInc)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint:nolintlint,errcheck
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "incident updated"})
}

type Component struct {
	ID         int                  `json:"id" uri:"id"`
	Attributes []ComponentAttribute `json:"attributes"`
	Name       string               `json:"name"`
}

type ComponentAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (a *App) GetComponentsStatusHandler(c *gin.Context) {
	r, err := a.DB.GetComponentsWithValues()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint:nolintlint,errcheck
		return
	}

	c.JSON(http.StatusOK, r)
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
func (a *App) PostComponentStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{"status": "in development"})
}
