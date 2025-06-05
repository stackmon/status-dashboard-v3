package rss

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
	"go.uber.org/zap"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const generalTitle = "Incidents | Status Dashboard"
const maxIncidents = 10

var errRSSWrongParams = fmt.Errorf( //nolint:staticcheck
	"Status Dashboard RSS feed\nPlease read the documentation to\nmake the correct request",
)

// Pay attention, the impact levels will be changed.
var impactRepresentation = map[int]string{ //nolint:gochecknoglobals
	0: "Scheduled maintenance",
	1: "Minor incident (i.e. performance impact)",
	2: "Major incident",
	3: "Service outage",
}

// getIncidentImpactsStr returns a string representation of the incident impact level.
// The numeric values (0,1,2,3) represent specific impact levels for the incident.
func getIncidentImpactsStr(impact int) string {
	return impactRepresentation[impact]
}

func HandleRSS(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		region := c.Query("mt")
		componentName := c.Query("srv")
		logger.Info("rss feed requested", zap.String("region", region), zap.String("component", componentName))

		if componentName != "" && region == "" {
			apiErrors.RaiseStatusNotFoundErr(c, errRSSWrongParams)
			return
		}

		if region != "" && !validateRegion(region) {
			apiErrors.RaiseStatusNotFoundErr(c, fmt.Errorf("the region '%s' is not valid", region))
			return
		}

		baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.Host)
		params := feedParams{
			region:        region,
			componentName: componentName,
		}

		incidents, err := getIncidents(dbInst, logger, params, maxIncidents)
		if err != nil {
			if componentName != "" {
				apiErrors.RaiseStatusNotFoundErr(c, err)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		feedTitle := prepareTitle(region, componentName)

		feed := &feeds.Feed{
			Title:       feedTitle,
			Link:        &feeds.Link{Href: baseURL + "/rss/" + c.Request.URL.RawQuery, Rel: "self"},
			Description: feedTitle + " - Incidents",
			Created:     time.Now(),
		}

		incidents = sortIncidents(incidents)
		var feedItems []*feeds.Item

		for _, incident := range incidents {
			item := createFeedItemsForIncident(incident, baseURL)
			feedItems = append(feedItems, item...)
		}

		feed.Items = feedItems

		rss, err := feed.ToRss()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.Header("Content-Type", "application/rss+xml")
		c.String(http.StatusOK, rss)
	}
}

// TODO: add list of valid regions to the config or to the db
func validateRegion(region string) bool {
	regions := [3]string{"EU-DE", "EU-NL", "Global"}
	for _, r := range regions {
		if r == region {
			return true
		}
	}
	return false
}

type feedParams struct {
	region        string
	componentName string
}

func getIncidents(dbInstance *db.DB, log *zap.Logger, params feedParams, maxIncidents int) ([]*db.Incident, error) {
	var incidents []*db.Incident
	var err error

	incParams := &db.IncidentsParams{LastCount: maxIncidents}

	switch {
	case params.componentName != "" && params.region != "":
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: params.region,
		}

		var component *db.Component
		component, err = dbInstance.GetComponentFromNameAttrs(params.componentName, attr)
		if err != nil {
			log.Error("failed to get component", zap.Error(err))
			return nil, err
		}

		incidents, err = dbInstance.GetIncidentsByComponentID(component.ID, incParams)
		if err != nil {
			return nil, err
		}
	case params.componentName == "" && params.region != "":
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: params.region,
		}

		incidents, err = dbInstance.GetIncidentsByComponentAttr(attr, incParams)
		if err != nil {
			return nil, err
		}
	default:
		incidents, err = dbInstance.GetIncidents(incParams)
		if err != nil {
			return nil, err
		}
	}

	return incidents, nil
}

func createFeedItemsForIncident(incident *db.Incident, baseURL string) []*feeds.Item {
	var feedItems []*feeds.Item

	impact := getIncidentImpactsStr(*incident.Impact)

	var title string
	if len(incident.Components) > 1 {
		title = fmt.Sprintf("Status change of multiple services to %s", impact)
	} else {
		title = fmt.Sprintf("%s status changed to %s", incident.Components[0].Name, impact)
	}

	var description strings.Builder
	startDate := *incident.StartDate
	description.WriteString(fmt.Sprintf("A %s was detected at %s UTC for ", impact, startDate.Format(time.DateTime)))

	for i := range len(incident.Components) {
		c := incident.Components[i]
		description.WriteString(fmt.Sprintf("%s in %s", c.Name, c.Region()))
		if i != len(incident.Components)-1 {
			description.WriteString(", ")
		} else {
			description.WriteString(".")
		}
	}

	// Create a general item for the incident
	item := &feeds.Item{
		Title:       title,
		Link:        &feeds.Link{Href: fmt.Sprintf("%s/incidents/%d", baseURL, incident.ID)},
		Created:     *incident.StartDate,
		Description: description.String(),
	}

	feedItems = append(feedItems, item)

	// add updates
	for _, s := range incident.Statuses {
		d := fmt.Sprintf("An update was provided at %s UTC for ", s.Timestamp.Format(time.DateTime))
		if len(incident.Components) > 1 {
			for _, c := range incident.Components {
				d += fmt.Sprintf("%s (%s),", c.Name, c.Region())
			}
			d = strings.TrimSuffix(d, ",")
		} else {
			d += fmt.Sprintf("%s in %s", incident.Components[0].Name, incident.Components[0].Region())
		}

		d += fmt.Sprintf(": %s - %s", s.Status, s.Text)

		upd := &feeds.Item{
			Title:       fmt.Sprintf("Update published for: %s", *incident.Text),
			Link:        &feeds.Link{Href: fmt.Sprintf("%s/incidents/%d", baseURL, incident.ID)},
			Created:     s.Timestamp,
			Description: d,
		}

		feedItems = append(feedItems, upd)
	}

	return feedItems
}

func prepareTitle(region string, component string) string {
	if region != "" {
		if component != "" {
			return fmt.Sprintf("%s (%s) - %s", component, region, generalTitle)
		}
		return fmt.Sprintf("%s - %s", region, generalTitle)
	}

	return generalTitle
}

func sortIncidents(incidents []*db.Incident) []*db.Incident {
	openIncidents := make([]*db.Incident, 0)
	closedIncidents := make([]*db.Incident, 0)

	for _, incident := range incidents {
		if incident.EndDate == nil {
			openIncidents = append(openIncidents, incident)
		} else {
			closedIncidents = append(closedIncidents, incident)
		}
	}

	sort.Slice(openIncidents, func(i, j int) bool {
		return openIncidents[i].StartDate.After(*openIncidents[j].StartDate)
	})
	sort.Slice(closedIncidents, func(i, j int) bool {
		return closedIncidents[i].EndDate.After(*closedIncidents[j].EndDate)
	})
	return append(openIncidents, closedIncidents...)
}
