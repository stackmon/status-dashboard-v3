package rss

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"

	rssErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func validateRegion(dbInstance *db.DB, c *gin.Context, region string) bool {
	supportedRegions, err := dbInstance.GetUniqueAttributeValues("region")
	if err != nil {
		rssErrors.RaiseRssGenerationErr(c, err)
		return false
	}

	for _, supportedRegion := range supportedRegions {
		if supportedRegion == region {
			return true
		}
	}

	c.String(http.StatusNotFound, fmt.Sprintf("%s is not a supported region", region))
	return false
}

type feedParams struct {
	region        string
	componentName string
	baseURL       string
}

func getIncidentsAndTitle(dbInstance *db.DB, params feedParams) ([]*db.Incident, string, error) {
	var incidents []*db.Incident
	var feedTitle string
	var err error

	switch {
	case params.componentName != "" && params.region != "":
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: params.region,
		}

		component, cErr := dbInstance.GetComponentFromNameAttrs(params.componentName, attr)
		if cErr != nil {
			return nil, "", fmt.Errorf("component '%s' not found in region '%s'", params.componentName, params.region)
		}

		incidents, err = dbInstance.GetIncidentsByComponentID(component.ID)
		if err != nil {
			return nil, "", err
		}
		feedTitle = component.Name + " (" + params.region + ")" + " | OTC Status Dashboard"

	case params.region != "" && params.componentName == "":
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: params.region,
		}

		incidents, err = dbInstance.GetIncidentsByComponentAttr(attr)
		if err != nil {
			return nil, "", err
		}
		feedTitle = params.region + " | OTC Status Dashboard"

	default:
		params := &db.IncidentsParams{IsOpened: false}
		incidents, err = dbInstance.GetIncidents(params)
		if err != nil {
			return nil, "", err
		}
		feedTitle = "OTC Status Dashboard"
	}

	return incidents, feedTitle, nil
}

func createFeedContent(incident *db.Incident, impactLevel string) string {
	var content string

	if len(incident.Statuses) > 0 {
		for i := len(incident.Statuses) - 1; i >= 0; i-- {
			status := incident.Statuses[i]
			content += fmt.Sprintf(
				"<small>%s</small><br><strong>%s - </strong>%s<br><br><br>",
				status.Timestamp.Format("2006-01-02 15:04 MST"),
				status.Status,
				status.Text,
			)
		}
	}

	content += fmt.Sprintf("Incident impact: %s<br>", impactLevel)
	content += fmt.Sprintf(
		"Start date: %s<br>",
		incident.StartDate.Format("2006-01-02 15:04:05 MST"),
	)
	if incident.EndDate != nil {
		content += fmt.Sprintf(
			"End date: %s<br>",
			incident.EndDate.Format("2006-01-02 15:04:05 MST"),
		)
	}
	content += "<br>We apologize for the inconvenience and will share an update once we have more information."
	return content
}

func createFeedItem(incident *db.Incident, baseURL string, impactLevel string) *feeds.Item {
	item := &feeds.Item{
		Title:   fmt.Sprintln(*incident.Text),
		Link:    &feeds.Link{Href: fmt.Sprintf("%s/incidents/%d", baseURL, incident.ID)},
		Created: *incident.StartDate,
	}

	item.Content = createFeedContent(incident, impactLevel)

	if len(incident.Statuses) > 0 {
		item.Updated = incident.Statuses[len(incident.Statuses)-1].Timestamp
	}
	if incident.EndDate != nil {
		item.Updated = *incident.EndDate
	} else {
		item.Updated = *incident.StartDate
	}

	return item
}

func Handler(c *gin.Context) {
	region := c.Query("mt")
	componentName := c.Query("srv")
	if componentName != "" && region == "" {
		c.String(
			http.StatusNotFound,
			"Status Dashboard RSS feed\nPlease read the documentation to\nmake the correct request",
		)
		return
	}

	dbInterface := c.MustGet("db")
	dbInstance, ok := dbInterface.(*db.DB)
	if !ok {
		c.String(http.StatusInternalServerError, "Internal server error: invalid database instance")
		return
	}

	if componentName != "" || region != "" {
		if !validateRegion(dbInstance, c, region) {
			return
		}
	}

	baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.Host)
	params := feedParams{
		region:        region,
		componentName: componentName,
		baseURL:       baseURL,
	}

	incidents, feedTitle, err := getIncidentsAndTitle(dbInstance, params)
	if err != nil {
		if componentName != "" {
			c.String(http.StatusNotFound, err.Error())
			return
		}
		rssErrors.RaiseRssGenerationErr(c, err)
		return
	}

	feed := &feeds.Feed{
		Title:       feedTitle,
		Link:        &feeds.Link{Href: baseURL + "/rss/" + c.Request.URL.RawQuery, Rel: "self"},
		Description: feedTitle + " - Incidents",
		Created:     time.Now(),
	}

	incidents = SortIncidents(incidents)
	//nolint:mnd
	if len(incidents) > 10 {
		incidents = incidents[:10]
	}

	feedItems := make([]*feeds.Item, 0, len(incidents))
	incidentImpacts := conf.GetIncidentImpacts()

	for _, incident := range incidents {
		impactLevel := "Unknown"
		if incident.Impact != nil {
			if impactData, exists := incidentImpacts[*incident.Impact]; exists {
				impactLevel = impactData.String
			}
		}

		item := createFeedItem(incident, baseURL, impactLevel)
		feedItems = append(feedItems, item)
	}

	feed.Items = feedItems

	rss, err := feed.ToAtom()
	if err != nil {
		rssErrors.RaiseRssGenerationErr(c, err)
		return
	}

	c.Header("Content-Type", "application/rss+xml")
	c.String(http.StatusOK, rss)
}

func SortIncidents(incidents []*db.Incident) []*db.Incident {
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
