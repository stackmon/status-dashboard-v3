package app

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *App) ValidateComponentsMW() gin.HandlerFunc {
	return func(c *gin.Context) {
		var incData IncidentData
		if err := c.ShouldBindBodyWithJSON(&incData); err != nil {
			c.AbortWithError( //nolint:nolintlint,errcheck
				http.StatusBadRequest,
				fmt.Errorf("%w: %w", ErrComponentValidation, err))
			return
		}

		// TODO: move this list to the memory cache
		// We should check, that all components are presented in our db.
		components, err := a.DB.GetComponents()
		if err != nil {
			c.AbortWithError( //nolint:nolintlint,errcheck
				http.StatusInternalServerError,
				fmt.Errorf("%w: %w", ErrComponentValidation, err),
			)
		}

		for _, comp := range incData.Components {
			var isPresent bool

			for _, dbComp := range components {
				if uint(comp) == dbComp.ID {
					isPresent = true
				}
			}
			if !isPresent {
				c.AbortWithError( //nolint:nolintlint,errcheck
					http.StatusBadRequest,
					fmt.Errorf("%w: component id %d is not presented", ErrComponentValidation, comp))
			}
		}

		c.Next()
	}
}

func ErrorHandle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		err := c.Errors.Last()
		if err == nil {
			return
		}

		c.JSON(-1, gin.H{
			"errorMsg": err.Error(),
		})
	}
}

func Return404(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"errorMsg": "page not found"})
}
