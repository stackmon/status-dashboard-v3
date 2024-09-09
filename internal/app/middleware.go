package app

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *App) ValidateComponentsMW() gin.HandlerFunc {
	return func(c *gin.Context) {

		type Components struct {
			Components []int `json:"components"`
		}

		var components Components

		if err := c.ShouldBindBodyWithJSON(&components); err != nil {
			c.AbortWithError( //nolint:nolintlint,errcheck
				http.StatusBadRequest,
				fmt.Errorf("%w: %w", ErrComponentIsNotPresent, err))
			return
		}

		// TODO: move this list to the memory cache
		// We should check, that all components are presented in our db.
		err := a.IsPresentComponent(components.Components)
		if err != nil {
			if errors.Is(err, ErrComponentIsNotPresent) {
				c.AbortWithError(http.StatusBadRequest, err)
			}
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		c.Next()
	}
}

func (a *App) IsPresentComponent(components []int) error {

	dbComps, err := a.DB.GetComponentsAsMap()
	if err != nil {
		return err
	}

	for _, comp := range components {
		if _, ok := dbComps[comp]; !ok {
			return ErrComponentIsNotPresent
		}
	}

	return nil
}

func ErrorHandle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		err := c.Errors.Last()
		if err == nil {
			return
		}

		c.JSON(-1, ReturnError(err))
	}
}

func Return404(c *gin.Context) {
	c.JSON(http.StatusNotFound, ReturnError(ErrPageNotFound))
}
