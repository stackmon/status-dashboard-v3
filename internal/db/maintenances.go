package db

import (
	"gorm.io/gorm"
)

func (db *DB) GetMaintenances(after uint) ([]*Incident, error) {
	var incidents []*Incident

	r := db.g.Model(&Incident{}).
		Preload("Statuses").
		Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Select("ID") })

	r.Where("incident.impact = 0")

	if after > 0 {
		r.Where("incident.id >= ?", after)
	}

	r = r.Order("incident.id DESC")

	if err := r.Find(&incidents).Error; err != nil {
		return nil, err
	}
	return incidents, nil
}
