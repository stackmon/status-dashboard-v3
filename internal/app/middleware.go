package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (a *App) ValidateComponentsMW() gin.HandlerFunc {
	return func(c *gin.Context) {
		type Components struct {
			Components []int `json:"components"`
		}

		var components Components

		if err := c.ShouldBindBodyWithJSON(&components); err != nil {
			raiseBadRequestErr(c, fmt.Errorf("%w: %w", ErrComponentInvalidFormat, err))
			return
		}

		// TODO: move this list to the memory cache
		// We should check, that all components are presented in our db.
		err := a.IsPresentComponent(components.Components)
		if err != nil {
			if errors.Is(err, ErrComponentDSNotExist) {
				raiseBadRequestErr(c, err)
			} else {
				raiseInternalErr(c, err)
			}
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
			return ErrComponentDSNotExist
		}
	}

	return nil
}

func ErrorHandle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}

		status := c.Writer.Status()

		var err error
		err = c.Errors.Last()
		if status >= http.StatusInternalServerError {
			err = ErrInternalError
		}

		c.JSON(-1, ReturnError(err))
	}
}

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now().UTC()
		latency := end.Sub(start)

		fields := []zapcore.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			msg := fmt.Sprintf("panic was recovered, %s", ErrInternalError)
			if c.Errors.Last() != nil {
				msg = c.Errors.Last().Error()
			}
			log.Error(msg, fields...)
		case c.Writer.Status() >= http.StatusBadRequest:
			for _, e := range c.Errors.Errors() {
				log.Info(e, fields...)
			}
		default:
			log.Info(path, fields...)
		}
	}
}
