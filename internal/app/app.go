package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const (
	readHeaderTimeout = 3 * time.Second
)

type App struct {
	// Configuration
	conf *conf.Config
	// Router
	router *gin.Engine
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
		Addr:              fmt.Sprintf(":%s", c.Port),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	a := &App{router: r, Log: log, conf: c, DB: d, srv: s}
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
