package db

import (
	"errors"
	"fmt"
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

	r := db.g.Model(&Incident{}).
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

// GetComponentFromNameAttrs returns the Component from its name and region attribute.
func (db *DB) GetComponentFromNameAttrs(name string, attr *ComponentAttr) (*Component, error) {
	comp := Component{}
	//nolint:lll
	// You can reproduce this raw request
	// select * from component join component_attribute ca on component.id=ca.component_id
	// where component.id =
	// (select component.id from component join component_attribute ca on component.id = ca.component_id and ca.value='EU-DE' and component.name='Cloud Container Engine');
	subQuery := db.g.Model(&Component{}).
		Select("component.id").
		Joins("JOIN component_attribute ca ON ca.component_id = component.id").
		Where("ca.value = ?", attr.Value).
		Where("component.name = ?", name)
	r := db.g.Model(&Component{}).Where("name = ?", name).
		Where("id = (?)", subQuery).
		Preload("Attrs").
		First(&comp)

	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			return nil, ErrDBComponentDSNotExist
		}
		return nil, r.Error
	}

	return &comp, nil
}

func (db *DB) SaveComponent(comp *Component) (uint, error) {
	var regIndex int
	for i, attr := range comp.Attrs {
		if attr.Name == "region" {
			regIndex = i
		}
	}

	_, err := db.GetComponentFromNameAttrs(comp.Name, &comp.Attrs[regIndex])
	if err == nil {
		return 0, ErrDBComponentExists
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	r := db.g.Create(comp)

	if r.Error != nil {
		return 0, r.Error
	}

	return comp.ID, nil
}

const statusSYSTEM = "SYSTEM"

func (db *DB) MoveComponentFromOldToAnotherIncident(
	comp *Component, incOld, incNew *Incident, closeOld bool,
) (*Incident, error) {
	timeNow := time.Now().UTC()

	if comp.Name == "" {
		c, err := db.GetComponent(int(comp.ID))
		if err != nil {
			return nil, err
		}
		comp = c
	}

	incNew.Components = append(incNew.Components, *comp)
	text := fmt.Sprintf("%s moved from %s", comp.PrintAttrs(), incOld.Link())
	incNew.Statuses = append(incNew.Statuses, IncidentStatus{
		IncidentID: incNew.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})

	text = fmt.Sprintf("%s moved to %s", comp.PrintAttrs(), incNew.Link())
	if closeOld {
		text = fmt.Sprintf("%s, Incident closed by system", text)
	}

	incOld.Statuses = append(incOld.Statuses, IncidentStatus{
		IncidentID: incOld.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})
	if closeOld {
		incOld.EndDate = &timeNow
	}

	err := db.g.Transaction(func(tx *gorm.DB) error {
		if !closeOld {
			if err := tx.Model(incOld).Association("Components").Delete(comp); err != nil {
				return err
			}
		}

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

func (db *DB) ExtractComponentToNewIncident(
	comp *Component, incOld *Incident, impact int, text string,
) (*Incident, error) {
	timeNow := time.Now().UTC()

	inc := &Incident{
		Text:       &text,
		StartDate:  &timeNow,
		EndDate:    nil,
		Impact:     &impact,
		Statuses:   []IncidentStatus{},
		System:     false,
		Components: []Component{*comp},
	}

	id, err := db.SaveIncident(inc)
	if err != nil {
		return nil, err
	}

	incText := fmt.Sprintf("%s moved from %s", comp.PrintAttrs(), incOld.Link())
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

	err = db.g.Model(incOld).Association("Components").Delete(comp)
	if err != nil {
		return nil, err
	}

	incText = fmt.Sprintf("%s moved to %s", comp.PrintAttrs(), inc.Link())
	incOld.Statuses = append(incOld.Statuses, IncidentStatus{
		IncidentID: inc.ID,
		Status:     statusSYSTEM,
		Text:       incText,
		Timestamp:  timeNow,
	})

	err = db.ModifyIncident(incOld)
	if err != nil {
		return nil, err
	}

	return inc, nil
}

func (db *DB) IncreaseIncidentImpact(inc *Incident, impact int) (*Incident, error) {
	timeNow := time.Now().UTC()
	text := fmt.Sprintf("impact changed from %d to %d", *inc.Impact, impact)
	inc.Statuses = append(inc.Statuses, IncidentStatus{
		IncidentID: inc.ID,
		Status:     statusSYSTEM,
		Text:       text,
		Timestamp:  timeNow,
	})
	inc.Impact = &impact

	if r := db.g.Updates(inc); r.Error != nil {
		return nil, r.Error
	}

	return inc, nil
}
