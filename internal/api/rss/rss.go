package rss

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"

	rssErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
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
		incident, err := dbInstance.GetOpenedIncidentsWithComponent(componentName, []db.ComponentAttr{*attr})
		if err != nil && err != db.ErrDBIncidentDSNotExist {
			rssErrors.RaiseRssGenerationErr(c, err)
			return
		}
		if incident != nil {
			incidents = append(incidents, incident)
		}

		feedTitle = "OTC Status Dashboard - " + component.Name + " (" + region + ")"
	} else {
		// Get all incidents if no component specified
		params := &db.IncidentsParams{IsOpened: true}
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

	feedItems := make([]*feeds.Item, 0, len(incidents))
	for _, incident := range incidents {
		var description string
		if incident.Text != nil {
			description = *incident.Text
		}

		item := &feeds.Item{
			Title:       fmt.Sprintf("Incident #%d - Impact Level %d", incident.ID, *incident.Impact),
			Link:        &feeds.Link{Href: fmt.Sprintf("https://status.otc-service.com/incidents/%d", incident.ID)},
			Description: description,
			Created:     *incident.StartDate,
		}

		if incident.EndDate != nil {
			item.Updated = *incident.EndDate
		}

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
