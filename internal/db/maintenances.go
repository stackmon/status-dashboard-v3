package db

import (
	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func (db *DB) GetMaintenances(after uint) ([]*Incident, error) {
	var incidents []*Incident

	r := db.g.Model(&Incident{}).
		Preload("Statuses").
		Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Select("ID") })

	r = r.Where("incident.type = ?", event.TypeMaintenance)

	if after > 0 {
		r = r.Where("incident.id >= ?", after)
	}

	r = r.Order("incident.id ASC")

	if err := r.Find(&incidents).Error; err != nil {
		return nil, err
	}
	return incidents, nil
}
