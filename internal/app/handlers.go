package app

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type incident struct {
	Id uint `uri:"id" binding:"required"`
}

func (a *App) GetIncidents(c *gin.Context) {
	r, err := a.DB.GetIncidents()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": r})
}

func (a *App) GetIncident(c *gin.Context) {
	var inc incident
	if err := c.ShouldBindUri(&inc); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) //nolint
		return
	}

	r, err := a.DB.GetIncident(inc.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err) //nolint
		return
	}

	c.JSON(http.StatusOK, r)
}
