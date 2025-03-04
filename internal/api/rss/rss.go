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

func RssHandler(c *gin.Context) {
	region := c.Query("mt")
	componentName := c.Query("srv")
	if componentName != "" && region == "" {
		c.String(
			http.StatusNotFound,
			"Status Dashboard RSS feed\nPlease read the documentation to\nmake the correct request",
		)
		return
	}

	// Get DB instance from gin context
	dbInterface := c.MustGet("db")
	dbInstance, ok := dbInterface.(*db.DB)
	if !ok {
		c.String(http.StatusInternalServerError, "Internal server error: invalid database instance")
		return
	}

	var incidents []*db.Incident
	var feedTitle string
	var err error

	switch {
	case componentName != "" && region != "":
		if !validateRegion(dbInstance, c, region) {
			return
		}

		attr := &db.ComponentAttr{
			Name:  "region",
			Value: region,
		}

		component, cErr := dbInstance.GetComponentFromNameAttrs(componentName, attr)
		if cErr != nil {
			c.String(http.StatusNotFound, "Component not found")
			return
		}

		// Get incidents for the component
		incidents, err = dbInstance.GetIncidentsByComponentID(component.ID)
		if err != nil {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		feedTitle = component.Name + " (" + region + ")" + " | OTC Status Dashboard"
	case region != "" && componentName == "":
		if !validateRegion(dbInstance, c, region) {
			return
		}

		// Get all incidents for the region
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: region,
		}

		incidents, err = dbInstance.GetIncidentsByComponentAttr(attr)
		if err != nil {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		feedTitle = region + " | OTC Status Dashboard"
	default:
		// Get all incidents if no component specified
		params := &db.IncidentsParams{IsOpened: false}
		incidents, err = dbInstance.GetIncidents(params)
		if err != nil {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		feedTitle = "OTC Status Dashboard"
	}

	// Get base URL from request
	baseURL := fmt.Sprintf("%s://%s", c.Request.URL.Scheme, c.Request.Host)

	var rssPath, queryParams, descriptionField string
	switch {
	case componentName != "" && region != "":
		rssPath = baseURL + "/rss/"
		queryParams = "?mt=" + region + "&srv=" + componentName
		descriptionField = fmt.Sprintf("%s (%s) - Incidents", componentName, region)
	case region != "":
		rssPath = baseURL + "/rss/"
		queryParams = "?mt=" + region
		descriptionField = fmt.Sprintf("%s - Incidents", region)
	default:
		rssPath = baseURL
		queryParams = ""
		descriptionField = "OTC Status Dashboard - Incidents"
	}

	feed := &feeds.Feed{
		Title: feedTitle,
		Link: &feeds.Link{
			Href: rssPath + queryParams,
			Rel:  "self",
		},
		Description: descriptionField,
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

		item := &feeds.Item{
			// ToDo: return impactLevel, incident.ID to Title
			Title:   fmt.Sprintln(*incident.Text),
			Link:    &feeds.Link{Href: fmt.Sprintf("%s/incidents/%d", baseURL, incident.ID)},
			Created: *incident.StartDate,
		}

		var content string

		if len(incident.Statuses) > 0 {
			// content += fmt.Sprintf(
			// 	"Last update: %s",
			// 	incident.Statuses[len(incident.Statuses)-1].Timestamp.Format("2006-01-02 15:04:05 MST"),
			// )
			// content += fmt.Sprintf("<br>Last status: %s", incident.Statuses[len(incident.Statuses)-1].Text)
			for i := len(incident.Statuses) - 1; i >= 0; i-- {
				status := incident.Statuses[i]
				content += fmt.Sprintf(
					"<small>%s</small><br><strong>%s - </strong>%s<br><br><br>",
					status.Timestamp.Format("2006-01-02 15:04 MST"),
					status.Status,
					status.Text,
				)
			}
			item.Updated = incident.Statuses[len(incident.Statuses)-1].Timestamp
		}

		content += fmt.Sprintf("Incident impact: %s<br>", impactLevel)
		content += fmt.Sprintf(
			"Incident has started on: %s<br>",
			incident.StartDate.Format("2006-01-02 15:04:05 MST"),
		)
		if incident.EndDate != nil {
			content += fmt.Sprintf(
				"End date: %s<br>",
				incident.EndDate.Format("2006-01-02 15:04:05 MST"),
			)
			item.Updated = *incident.EndDate
		} else {
			item.Updated = *incident.StartDate
		}
		content += "<br>We apologize for the inconvenience and will share an update once we have more information."
		item.Content = content
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
