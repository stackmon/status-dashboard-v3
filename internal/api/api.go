package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

type API struct {
	r   *gin.Engine
	db  *db.DB
	log *zap.Logger
}

func New(cfg *conf.Config, log *zap.Logger, database *db.DB) *API {
	if cfg.LogLevel != conf.DevelopMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(Logger(log), gin.Recovery())
	r.Use(ErrorHandle())
	r.Use(CORSMiddleware())
	r.NoRoute(errors.Return404)

	a := &API{r: r, db: database, log: log}
	a.initRoutes()
	return a
}

func (a *API) Router() *gin.Engine {
	return a.r
}
