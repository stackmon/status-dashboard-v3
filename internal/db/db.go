package db

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"moul.io/zapgorm2"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
)

type DB struct {
	g *gorm.DB
}

func New(c *conf.Config) (*DB, error) {
	psql := postgres.New(postgres.Config{
		DSN: c.DB,
	})

	gConf := &gorm.Config{}

	if c.LogLevel != conf.DevelopMode {
		logger := zapgorm2.New(zap.L())
		gConf.Logger = logger
	}

	g, err := gorm.Open(psql, gConf)
	if err != nil {
		return nil, err
	}

	return &DB{g: g}, nil
}

type IncidentsParams struct {
	IsOpened bool
}

func (db *DB) GetIncidents(params ...*IncidentsParams) ([]*Incident, error) {
	var incidents []*Incident
	var param IncidentsParams
	if params != nil && params[0] != nil {
		param = *params[0]
	}

	r := db.g.Debug().Model(&Incident{}).
		Preload("Statuses").
		Preload("Components", func(db *gorm.DB) *gorm.DB { return db.Select("ID") })

	if param.IsOpened {
		r.Where("end_date is NULL").Find(&incidents)
		if r.Error != nil {
			return nil, r.Error
		}
		return incidents, nil
	}

	r.Find(&incidents)

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
		Preload("Components", func(db *gorm.DB) *gorm.DB {
			return db.Select("ID")
		}).
		First(&inc)

	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			return nil, ErrDBIncidentDSNotExist
		}
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

// TODO: check this function for patching incident
func (db *DB) ModifyIncident(inc *Incident) error {
	r := db.g.Updates(inc)

	if r.Error != nil {
		return r.Error
	}

	return nil
}

func (db *DB) GetOpenedIncidentsWithComponent(name string, attrs []ComponentAttr) (*Incident, error) {
	comp := &Component{Name: name, Attrs: attrs}
	r := db.g.Model(&Component{}).Preload("Attrs").Find(comp)
	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			return nil, ErrDBComponentDSNotExist
		}
		return nil, r.Error
	}

	var incident Incident
	r = db.g.Model(&Incident{}).
		Preload("Statuses").
		Preload("Components", func(db *gorm.DB) *gorm.DB {
			return db.Select("ID")
		}).
		// Where("component_id = ?", comp.ID).
		First(&incident)

	if r.Error != nil {
		return nil, r.Error
	}

	return &incident, nil
}

func (db *DB) GetComponent(id int) (*Component, error) {
	comp := &Component{ID: uint(id)}
	r := db.g.Model(&Component{}).Preload("Attrs").First(comp)

	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			return nil, ErrDBComponentDSNotExist
		}
		return nil, r.Error
	}

	return comp, nil
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

func (db *DB) GetComponentsWithIncidents() ([]Component, error) {
	var components []Component
	r := db.g.Model(&Component{}).Preload("Attrs").Preload("Incidents").Preload("Incidents.Statuses").Find(&components)

	if r.Error != nil {
		return nil, r.Error
	}

	return components, nil
}

func (db *DB) GetComponentFromNameAttrs(name string, attrs *ComponentAttr) (*Component, error) {
	comp := Component{}
	//nolint:lll
	// You can reproduce this raw request
	// select * from component join component_attribute ca on component.id=ca.component_id
	// where component.id =
	// (select component.id from component join component_attribute ca on component.id = ca.component_id and ca.value='EU-DE' and component.name='Cloud Container Engine');
	r := db.g.Model(&Component{}).
		Where(&Component{Name: name}).
		Preload("Attrs", func(db *gorm.DB) *gorm.DB {
			return db.Where("name=?", attrs.Name).Where("value=?", attrs.Value)
		}).
		Preload("Attrs").Find(&comp)

	if r.Error != nil {
		return nil, r.Error
	}

	return &comp, nil
}

const statusSYSTEM = "SYSTEM"

func (db *DB) MoveComponentFromOldToAnotherIncident(comp *Component, incOld, incNew *Incident) (*Incident, error) {
	timeNow := time.Now()

	incNew.Components = append(incNew.Components, *comp)
	text := fmt.Sprintf("%s moved from %s", comp.PrintAttrs(), incOld.Link())
	incNew.Statuses = append(incNew.Statuses, IncidentStatus{
		IncidentID: incNew.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})

	var indexToRemove int
	for i, c := range incOld.Components {
		if c.ID == comp.ID {
			indexToRemove = i
		}
	}
	incOld.Components = slices.Delete(incOld.Components, indexToRemove, indexToRemove+1)
	text = fmt.Sprintf("%s moved to %s", comp.PrintAttrs(), incNew.Link())
	incOld.Statuses = append(incOld.Statuses, IncidentStatus{
		IncidentID: incOld.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})

	err := db.g.Transaction(func(tx *gorm.DB) error {
		if r := tx.Save(incNew); r.Error != nil {
			return r.Error
		}
		if r := tx.Save(incOld); r.Error != nil {
			return r.Error
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return incNew, nil
}

func (db *DB) AddComponentToNewIncidentAndCloseOld(comp *Component, incToClose, inc *Incident) (*Incident, error) {
	if len(inc.Components) == 0 {
		var comps []Component
		inc.Components = comps
	}
	inc.Components = append(inc.Components, *comp)

	timeNow := time.Now()

	text := fmt.Sprintf("%s moved from %s", comp.PrintAttrs(), inc.Link())
	inc.Statuses = append(inc.Statuses, IncidentStatus{
		IncidentID: inc.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})

	closedText := fmt.Sprintf("%s moved to %s, Incident closed by system", comp.PrintAttrs(), inc.Link())
	incToClose.Statuses = append(incToClose.Statuses, IncidentStatus{
		IncidentID: incToClose.ID,
		Status:     statusSYSTEM,
		Text:       closedText,
		Timestamp:  timeNow,
	})
	incToClose.EndDate = &timeNow

	err := db.g.Transaction(func(tx *gorm.DB) error {
		if r := tx.Save(inc); r.Error != nil {
			return r.Error
		}
		if r := tx.Save(incToClose); r.Error != nil {
			return r.Error
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return inc, nil
}

func (db *DB) ExtractComponentToNewIncident(
	comp *Component, oldIncident *Incident, impact int, text string,
) (*Incident, error) {
	timeNow := time.Now()

	inc := &Incident{
		Text:       &text,
		StartDate:  &timeNow,
		EndDate:    nil,
		Impact:     &impact,
		Statuses:   nil,
		System:     false,
		Components: []Component{*comp},
	}

	id, err := db.SaveIncident(inc)
	if err != nil {
		return nil, err
	}

	incText := fmt.Sprintf("%s moved from %s", comp.PrintAttrs(), oldIncident.Link())
	inc.Statuses = append(inc.Statuses, IncidentStatus{
		IncidentID: id,
		Status:     statusSYSTEM,
		Text:       incText,
		Timestamp:  timeNow,
	})

	err = db.ModifyIncident(inc)
	if err != nil {
		return nil, err
	}

	var indexToRemove int
	for i, c := range oldIncident.Components {
		if c.ID == comp.ID {
			indexToRemove = i
		}
	}
	oldIncident.Components = slices.Delete(oldIncident.Components, indexToRemove, indexToRemove+1)
	incText = fmt.Sprintf("%s moved to %s", comp.PrintAttrs(), inc.Link())
	oldIncident.Statuses = append(oldIncident.Statuses, IncidentStatus{
		IncidentID: inc.ID,
		Status:     statusSYSTEM,
		Text:       incText,
		Timestamp:  timeNow,
	})

	err = db.ModifyIncident(oldIncident)
	if err != nil {
		return nil, err
	}

	return inc, nil
}

func (db *DB) IncreaseIncidentImpact(inc *Incident, impact int) (*Incident, error) {
	timeNow := time.Now()
	text := fmt.Sprintf("impact changed from %d to %d", inc.Impact, impact)
	inc.Statuses = append(inc.Statuses, IncidentStatus{
		IncidentID: inc.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})
	inc.Impact = &impact

	if r := db.g.Save(inc); r.Error != nil {
		return nil, r.Error
	}

	return inc, nil
}
