package db

import (
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	g *gorm.DB
}

func New(c *conf.Config) (*DB, error) {
	psql := postgres.New(postgres.Config{
		DSN: c.DB,
	})
	g, err := gorm.Open(psql)
	if err != nil {
		return nil, err
	}

	return &DB{g: g}, nil
}

func (db *DB) GetIncidents() ([]Incident, error) {
	var incidents []Incident
	r := db.g.Model(&Incident{}).Preload("Statuses").Find(&incidents)

	if r.Error != nil {
		return nil, r.Error
	}
	return incidents, nil
}

func (db *DB) GetIncident(id int) (*Incident, error) {
	inc := Incident{Id: uint(id)}

	r := db.g.Model(&Incident{}).Where(inc).Preload("Statuses").Preload("Components").Find(&inc)

	if r.Error != nil {
		return nil, r.Error
	}

	return &inc, nil
}

func (db *DB) SaveIncident(inc *Incident) (uint, error) {
	r := db.g.Debug().Create(inc)

	if r.Error != nil {
		return 0, r.Error
	}

	return inc.Id, nil
}

func (db *DB) GetComponents() ([]Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	return components, nil
}

func (db *DB) GetComponentsWithValues() ([]Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Preload("Attrs").Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	return components, nil
}
