package db

import (
	"fmt"
	"time"

	"github.com/stackmon/otc-status-dashboard/internal/event"
)

type Component struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name,omitempty"`
	Attrs     []ComponentAttr `json:"attributes,omitempty"`
	Incidents []*Incident     `json:"incidents,omitempty" gorm:"many2many:incident_component_relation"`
}

func (c *Component) TableName() string {
	return "component"
}

func (c *Component) PrintAttrs() string {
	var category, region, compType string
	for _, a := range c.Attrs {
		switch a.Name {
		case "category":
			category = a.Value
		case regionAttrName:
			region = a.Value
		case "type":
			compType = a.Value
		}
	}
	return fmt.Sprintf("%s (%s, %s, %s)", c.Name, category, region, compType)
}

func (c *Component) Region() string {
	var region string
	for _, a := range c.Attrs {
		if a.Name == regionAttrName {
			region = a.Value
		}
	}

	return region
}

func (c *Component) Type() string {
	var cType string
	for _, a := range c.Attrs {
		if a.Name == "type" {
			cType = a.Value
		}
	}

	return cType
}

const regionAttrName = "region"

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
	ID           uint             `json:"id"`
	Text         *string          `json:"text" gorm:"not null"`
	Description  *string          `json:"description" gorm:"type:varchar(500)"`
	StartDate    *time.Time       `json:"start_date" gorm:"not null"`
	EndDate      *time.Time       `json:"end_date"`
	Impact       *int             `json:"impact" gorm:"not null"`
	Statuses     []IncidentStatus `json:"updates" gorm:"foreignKey:IncidentID"`
	ActualStatus *string          `json:"actual_status" gorm:"type:varchar(50)"`
	System       bool             `json:"system" gorm:"not null"`
	Type         string           `json:"type" gorm:"not null"`
	Components   []Component      `json:"components" gorm:"many2many:incident_component_relation"`
}

func (in *Incident) TableName() string {
	return "incident"
}

func (in *Incident) Link() string {
	return fmt.Sprintf("<a href='/incidents/%d'>%s</a>", in.ID, *in.Text)
}

// IncidentStatus is a db table representation.
type IncidentStatus struct {
	ID         uint         `json:"-" gorm:"primaryKey;autoIncrement:true;"`
	IncidentID uint         `json:"-"`
	Status     event.Status `json:"status"`
	Text       string       `json:"text"`
	Timestamp  time.Time    `json:"timestamp"`
}

func (is *IncidentStatus) TableName() string {
	return "incident_status"
}
