package db

import (
	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func (db *DB) GetInfoEvents(after uint) ([]*Incident, error) {
	var incidents []*Incident

	r := db.g.Model(&Incident{}).
		Preload("Statuses").
		Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Select("ID") })

	r.Where("incident.type = ?", event.TypeInformation)

	if after > 0 {
		r.Where("incident.id >= ?", after)
	}

	r = r.Order("incident.id DESC")

	if err := r.Find(&incidents).Error; err != nil {
		return nil, err
	}
	return incidents, nil
}
