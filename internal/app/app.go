package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api"
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
	api *api.API
	// zap logger
	Log *zap.Logger
	// db connection
	DB *db.DB
	// http server
	srv *http.Server
}

func New(c *conf.Config, log *zap.Logger) (*App, error) {
	dbNew, err := db.New(c)
	if err != nil {
		return nil, err
	}

	apiNew, err := api.New(c, log, dbNew)
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		Addr:              fmt.Sprintf(":%s", c.Port),
		Handler:           apiNew.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	a := &App{api: apiNew, Log: log, conf: c, DB: dbNew, srv: s}

	return a, nil
}

func (a *App) Run() error {
	return a.srv.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	// TODO: add a proper shutdown for a database
	return a.srv.Shutdown(ctx)
}
