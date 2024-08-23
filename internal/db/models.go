package db

import (
	"time"
)

// Incident is a db table representation.
// CREATE TABLE public.incident (
//
//	id integer NOT NULL,
//	text character varying NOT NULL,
//	start_date timestamp without time zone NOT NULL,
//	end_date timestamp without time zone,
//	impact smallint NOT NULL,
//	system boolean DEFAULT false NOT NULL
//
// );
type Incident struct {
	Id        uint             `json:"id"`
	Text      string           `json:"text"`
	StartDate time.Time        `json:"start_date"`
	EndDate   *time.Time       `json:"end_date"`
	Impact    uint8            `json:"impact"`
	Updates   []IncidentStatus `json:"updates" gorm:"foreignKey:IncidentId"`
	System    bool             `json:"-"`
}

func (in *Incident) TableName() string {
	return "incident"
}

// IncidentStatus is a db table representation.
// CREATE TABLE public.incident_status (
//
//	id integer NOT NULL,
//	incident_id integer,
//	"timestamp" timestamp without time zone NOT NULL,
//	text character varying NOT NULL,
//	status character varying NOT NULL
//
// );
type IncidentStatus struct {
	Id         uint
	IncidentId uint
	Status     string    `json:"status"`
	Text       string    `json:"text"`
	Timestamp  time.Time `json:"timestamp"`
}

func (is *IncidentStatus) TableName() string {
	return "incident_status"
}
