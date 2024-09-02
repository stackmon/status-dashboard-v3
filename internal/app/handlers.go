package app

import (
	"github.com/gin-gonic/gin"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"net/http"
	"time"
)

type IncidentId struct {
	Id int `json:"id" uri:"id" binding:"required,gte=0"`
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
	IncidentId
	IncidentData
}

func (a *App) GetIncidentsHandler(c *gin.Context) {
	r, err := a.DB.GetIncidents()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": r})
}

func (a *App) GetIncidentHandler(c *gin.Context) {
	var incId IncidentId
	if err := c.ShouldBindUri(&incId); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint
		return
	}

	r, err := a.DB.GetIncident(incId.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint
		return
	}

	components := make([]int, len(r.Components))
	for i, comp := range r.Components {
		components[i] = int(comp.Id)
	}

	incData := IncidentData{
		Title:      r.Text,
		Impact:     &r.Impact,
		Components: components,
		StartDate:  r.StartDate,
		EndDate:    r.EndDate,
		System:     r.System,
		Updates:    r.Statuses,
	}

	c.JSON(http.StatusOK, &Incident{incId, incData})
}

func (a *App) PostIncidentHandler(c *gin.Context) {
	var incData IncidentData
	if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint
		return
	}

	components := make([]db.Component, len(incData.Components))
	for i, comp := range incData.Components {
		components[i] = db.Component{Id: uint(comp)}
	}

	dbInc := db.Incident{
		Text:       incData.Title,
		StartDate:  incData.StartDate,
		EndDate:    incData.EndDate,
		Impact:     *incData.Impact,
		System:     incData.System,
		Components: components,
	}

	incidentId, err := a.DB.SaveIncident(&dbInc)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint
		return
	}

	c.JSON(http.StatusOK, Incident{
		IncidentId:   IncidentId{int(incidentId)},
		IncidentData: incData,
	})
}
