package common

import (
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func MoveIncidentToHigherImpact(
	dbInst *db.DB, log *zap.Logger,
	storedComponent *db.Component, incident *db.Incident, incidents []*db.Incident,
	impact int, text string,
) (*db.Incident, error) {
	incWithHighImpact := FindIncidentByImpact(impact, incidents)
	if incWithHighImpact == nil {
		if len(incident.Components) > 1 {
			log.Info("no active incidents with requested impact, opening the new one")
			components := []db.Component{*storedComponent}
			return dbInst.ExtractComponentsToNewIncident(components, incident, impact, text)
		}
		log.Info(
			"only one component in the incident, increase impact",
			zap.Intp("oldImpact", incident.Impact),
			zap.Int("newImpact", impact),
		)
		return dbInst.IncreaseIncidentImpact(incident, impact)
	}

	if len(incident.Components) == 1 {
		log.Info("move component to the incident with the found impact, close current incident")
		return dbInst.MoveComponentFromOldToAnotherIncident(storedComponent, incident, incWithHighImpact, true)
	}

	// In that case we have the existed incident with target impact (greater where component is presented)
	// And count of components is more than one. We should move component from old to new.
	log.Info("move component to the incident with the higher impact")
	return dbInst.MoveComponentFromOldToAnotherIncident(storedComponent, incident, incWithHighImpact, false)
}

func FindIncidentByImpact(impact int, incidents []*db.Incident) *db.Incident {
	for _, incident := range incidents {
		if *incident.Impact == impact {
			return incident
		}
	}
	return nil
}

func GetIncidentWithComponent(componentID uint, incidents []*db.Incident) *db.Incident {
	for i, incident := range incidents {
		for _, component := range incident.Components {
			if componentID == component.ID {
				return incidents[i]
			}
		}
	}

	return nil
}
