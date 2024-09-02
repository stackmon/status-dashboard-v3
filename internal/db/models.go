package db

import (
	"time"
)

type Component struct {
	ID    uint            `json:"id" gorm:"many2many:incident_component_relation"`
	Name  string          `json:"name"`
	Attrs []ComponentAttr `json:"attrs"`
}

func (c *Component) TableName() string {
	return "component"
}

type ComponentAttr struct {
	ID          uint   `json:"-"`
	ComponentID uint   `json:"-"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

func (c *ComponentAttr) TableName() string {
	return "component_attribute"
}

// Incident is a db table representation.
type Incident struct {
	ID         uint             `json:"id"`
	Text       string           `json:"text"`
	StartDate  time.Time        `json:"start_date"`
	EndDate    *time.Time       `json:"end_date"`
	Impact     int              `json:"impact"`
	Statuses   []IncidentStatus `json:"updates"`
	System     bool             `json:"system"`
	Components []Component      `json:"components" gorm:"many2many:incident_component_relation"`
}

func (in *Incident) TableName() string {
	return "incident"
}

// IncidentStatus is a db table representation.
type IncidentStatus struct {
	ID         uint      `json:"id"`
	IncidentID uint      `json:"-"`
	Status     string    `json:"status"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
}

func (is *IncidentStatus) TableName() string {
	return "incident_status"
}
