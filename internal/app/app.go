package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stackmon/otc-status-dashboard/internal/api"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/notification"
)

const (
	readHeaderTimeout = 3 * time.Second
	// notificationStaleThreshold flags a processing row as stuck in the /metrics gauge.
	notificationStaleThreshold = 2 * time.Minute
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
	// notification delivery worker (nil when notifications are disabled)
	worker       *notification.Worker
	workerCancel context.CancelFunc
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

	// Build the delivery worker on the app's shared pool — it must NOT open its own.
	// Budget PostgreSQL connections as max_open_conns_per_pod * number_of_pods.
	worker, err := buildWorker(c, log, dbNew, apiNew)
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		Addr:              fmt.Sprintf(":%s", c.Port),
		Handler:           apiNew.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	a := &App{api: apiNew, Log: log, conf: c, DB: dbNew, srv: s, worker: worker}

	return a, nil
}

// buildWorker constructs the delivery worker, registers Prometheus metrics on a
// /metrics endpoint, and wires the API publisher's hot-path signal to it. It returns
// nil (no worker) when notifications are disabled.
func buildWorker(c *conf.Config, log *zap.Logger, dbNew *db.DB, apiNew *api.API) (*notification.Worker, error) {
	ncfg, err := notification.ConfigFromConf(c)
	if err != nil {
		return nil, err
	}
	if !ncfg.Enabled {
		return nil, nil //nolint:nilnil // no worker when the feature is off
	}

	sender, err := notification.NewSMTPSender(ncfg)
	if err != nil {
		return nil, fmt.Errorf("build smtp sender: %w", err)
	}

	// Dedicated registry so /metrics exposes just the notification signals.
	reg := prometheus.NewRegistry()
	metrics := notification.NewMetrics()
	metrics.MustRegister(reg)
	reg.MustRegister(notification.NewStatsCollector(dbNew, notificationStaleThreshold))
	apiNew.Router().GET("/metrics", gin.WrapH(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))

	worker, err := notification.NewWorker(ncfg, dbNew, sender, log, metrics)
	if err != nil {
		return nil, fmt.Errorf("build notification worker: %w", err)
	}

	apiNew.Publisher().SetNotify(worker.Notify)
	return worker, nil
}

// NotifyFunc returns the worker's wake-up callback, or nil when notifications are
// disabled. Used to wire the checker's publisher to the same worker.
func (a *App) NotifyFunc() func() {
	if a.worker == nil {
		return nil
	}
	return a.worker.Notify
}

func (a *App) Run() error {
	if a.worker != nil {
		var ctx context.Context
		ctx, a.workerCancel = context.WithCancel(context.Background())
		go a.worker.Run(ctx)
	}
	return a.srv.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.workerCancel != nil {
		a.workerCancel()
	}
	// TODO: add a proper shutdown for a database
	return a.srv.Shutdown(ctx)
}
