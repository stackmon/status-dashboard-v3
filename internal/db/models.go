package db

import (
	"fmt"
	"time"
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
		case "region":
			region = a.Value
		case "type":
			compType = a.Value
		}
	}
	return fmt.Sprintf("%s (%s, %s, %s)", c.Name, category, region, compType)
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
	Text       *string          `json:"text" gorm:"not null"`
	StartDate  *time.Time       `json:"start_date" gorm:"not null"`
	EndDate    *time.Time       `json:"end_date"`
	Impact     *int             `json:"impact" gorm:"not null"`
	Statuses   []IncidentStatus `json:"updates" gorm:"foreignKey:IncidentID"`
	System     bool             `json:"system" gorm:"not null"`
	Components []Component      `json:"components" gorm:"many2many:incident_component_relation"`
}

func (in *Incident) TableName() string {
	return "incident"
}

func (in *Incident) Link() string {
	return fmt.Sprintf("<a href='/incidents/%d'>%s</a>", in.ID, *in.Text)
}

// IncidentStatus is a db table representation.
type IncidentStatus struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement:true;"`
	IncidentID uint      `json:"-"`
	Status     string    `json:"status"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
}

func (is *IncidentStatus) TableName() string {
	return "incident_status"
}
