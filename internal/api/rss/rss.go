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
	dbInstance := c.MustGet("db").(*db.DB)

	var incidents []*db.Incident
	var feedTitle string
	var err error

	if componentName != "" && region != "" {
		attr := &db.ComponentAttr{
			Name:  "region",
			Value: region,
		}

		component, err := dbInstance.GetComponentFromNameAttrs(componentName, attr)
		if err != nil {
			c.String(http.StatusNotFound, "Component not found")
			return
		}

		// Get incidents for the component
		incidents, err = dbInstance.GetIncidentsByComponentID(component.ID)
		if err != nil {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		feedTitle = "OTC Status Dashboard - " + component.Name + " (" + region + ")"
	} else if region != "" && componentName == "" {
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
		feedTitle = "OTC Status Dashboard - " + region
	} else {
		// Get all incidents if no component specified
		params := &db.IncidentsParams{IsOpened: false}
		incidents, err = dbInstance.GetIncidents(params)
		if err != nil {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		feedTitle = "OTC Status Dashboard - All Components"
	}

	feed := &feeds.Feed{
		Title:       feedTitle,
		Link:        &feeds.Link{Href: "https://status.otc-service.com/"},
		Description: "Open Telekom Cloud Status Dashboard",
		Created:     time.Now(),
	}

	incidents = SortIncidents(incidents)
	if len(incidents) > 10 {
		incidents = incidents[:10]
	}

	feedItems := make([]*feeds.Item, 0, len(incidents))
	for _, incident := range incidents {
		impactLevel := "Unknown"
		if incident.Impact != nil {
			if impactData, ok := conf.IncidentImpacts[*incident.Impact]; ok {
				impactLevel = impactData.String
			}
		}

		item := &feeds.Item{
			Title:   fmt.Sprintf("%s - Impact Level: %s #%d", *incident.Text, impactLevel, incident.ID),
			Link:    &feeds.Link{Href: fmt.Sprintf("https://status.otc-service.com/incidents/%d", incident.ID)},
			Created: *incident.StartDate,
		}

		var description string
		description = "<![CDATA[" + "<br>"
		description += fmt.Sprintf("Incident impact: %s<br>", impactLevel)
		description += fmt.Sprintf(
			"Incident has started on: %s<br>",
			incident.StartDate.Format("2006-01-02 15:04:05 MST"),
		)

		if incident.EndDate != nil {
			description += fmt.Sprintf(
				"End date: %s<br>",
				incident.EndDate.Format("2006-01-02 15:04:05 MST"),
			)
			item.Updated = *incident.EndDate
		} else {
			item.Updated = *incident.StartDate
		}

		if len(incident.Statuses) > 0 {
			description += fmt.Sprintf(
				"Last update: %s",
				incident.Statuses[len(incident.Statuses)-1].Timestamp.Format("2006-01-02 15:04:05 MST"),
			)
			description += fmt.Sprintf("<br>Last status: %s", incident.Statuses[len(incident.Statuses)-1].Text)
			item.Updated = incident.Statuses[len(incident.Statuses)-1].Timestamp
		}
		item.Description = description

		feedItems = append(feedItems, item)
	}

	feed.Items = feedItems

	rss, err := feed.ToRss()
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
