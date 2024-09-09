package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
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
	r := db.g.Model(&Incident{}).
		Preload("Statuses").
		Preload(
			"Components", func(db *gorm.DB) *gorm.DB {
				return db.Select("ID")
			}).
		Find(&incidents)

	if r.Error != nil {
		return nil, r.Error
	}
	return incidents, nil
}

func (db *DB) GetIncident(id int) (*Incident, error) {
	inc := Incident{ID: uint(id)}

	r := db.g.Model(&Incident{}).
		Where(inc).
		Preload("Statuses").
		Preload(
			"Components", func(db *gorm.DB) *gorm.DB {
				return db.Select("ID")
			}).
		Find(&inc)

	if r.Error != nil {
		return nil, r.Error
	}

	return &inc, nil
}

func (db *DB) SaveIncident(inc *Incident) (uint, error) {
	r := db.g.Create(inc)

	if r.Error != nil {
		return 0, r.Error
	}

	return inc.ID, nil
}

func (db *DB) ModifyIncident(inc *Incident) error {
	var components []Component

	if inc.Components != nil {
		components = inc.Components
		inc.Components = nil
	}

	r := db.g.Updates(inc)

	if components != nil { //nolint:revive,staticcheck,nolintlint
		// TODO: update components here
	}

	if r.Error != nil {
		return r.Error
	}

	return nil
}

func (db *DB) GetComponents() ([]Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	return components, nil
}

func (db *DB) GetComponentsAsMap() (map[int]*Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	var compMap = make(map[int]*Component)
	for _, comp := range components {
		compMap[int(comp.ID)] = &comp
	}

	return compMap, nil
}

func (db *DB) GetComponentsWithValues() ([]Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Preload("Attrs").Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	return components, nil
}
