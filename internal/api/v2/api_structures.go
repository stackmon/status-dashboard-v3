package v2

import (
	"time"

	"github.com/stackmon/otc-status-dashboard/internal/event"
)

// Event IDs and core data structures.
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
}

type PostIncidentResp struct {
	Result []*ProcessComponentResp `json:"result"`
}

type ProcessComponentResp struct {
	ComponentID int    `json:"component_id"`
	IncidentID  int    `json:"incident_id,omitempty"`
	Error       string `json:"error,omitempty"`
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

type PostIncidentSeparateData struct {
	Components []int `json:"components" binding:"required,min=1"`
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

type PostComponentData struct {
	Attributes []ComponentAttribute `json:"attrs" binding:"required"`
	Name       string               `json:"name" binding:"required"`
}

type EventUpdateData struct {
	Index     int          `json:"index"`
	ID        int          `json:"id,omitempty"`
	Status    event.Status `json:"status"`
	Text      string       `json:"text"`
	Timestamp time.Time    `json:"timestamp"`
}
type PatchEventUpdateData struct {
	IncidentID  int     `uri:"id" binding:"required,gte=0"`
	UpdateIndex int     `uri:"update_id" binding:"required,gte=0"`
	Text        *string `json:"text,omitempty"`
}
