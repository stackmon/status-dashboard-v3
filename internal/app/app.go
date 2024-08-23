package app

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"go.uber.org/zap"
	"net/http"
)

type App struct {
	// Configuration
	c *conf.Config
	// Router
	r *gin.Engine
	// zap logger
	Log *zap.Logger
	// db connection
	DB *db.DB
	// http server
	srv *http.Server
}

func New(c *conf.Config, log *zap.Logger) (*App, error) {
	if c.LogLevel != conf.DevelopMode {
		gin.SetMode(gin.ReleaseMode)
	}

	d, err := db.New(c)
	if err != nil {
		return nil, err
	}

	r := gin.Default()
	r.Use(ErrorHandle())
	r.NoRoute(Return404)

	s := &http.Server{
		Addr:    fmt.Sprintf(":%s", c.Port),
		Handler: r,
	}

	a := &App{r: r, Log: log, c: c, DB: d, srv: s}
	a.InitRoutes()
	return a, nil
}

func (a *App) Run() error {
	return a.srv.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	// TODO: add a proper shutdown for a database
	return a.srv.Shutdown(ctx)
}
