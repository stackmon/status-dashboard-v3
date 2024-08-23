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
	r := db.g.Model(&Incident{}).Preload("Updates").Find(&incidents)

	if r.Error != nil {
		return nil, r.Error
	}
	return incidents, nil
}

func (db *DB) GetIncident(id uint) (*Incident, error) {
	inc := Incident{Id: id}

	r := db.g.Model(&Incident{}).Where(inc).Preload("Updates").Find(&inc)

	if r.Error != nil {
		return nil, r.Error
	}

	return &inc, nil
}
