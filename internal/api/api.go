package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

type API struct {
	r       *gin.Engine
	db      *db.DB
	log     *zap.Logger
	oa2Prov *auth.Provider
}

func New(cfg *conf.Config, log *zap.Logger, database *db.DB) (*API, error) {
	if cfg.LogLevel != conf.DevelopMode {
		gin.SetMode(gin.ReleaseMode)
	}

	oa2Prov := &auth.Provider{Disabled: true}

	var hostURI string
	if !cfg.AuthenticationDisabled {
		if cfg.Port == "443" || cfg.Port == "80" {
			hostURI = cfg.Hostname
		} else {
			hostURI = fmt.Sprintf("%s:%s", cfg.Hostname, cfg.Port)
		}

		if cfg.SSLDisabled {
			hostURI = fmt.Sprintf("http://%s", hostURI)
		} else {
			hostURI = fmt.Sprintf("https://%s", hostURI)
		}

		var err error
		oa2Prov, err = auth.NewProvider(
			cfg.Keycloak.URL, cfg.Keycloak.Realm, cfg.Keycloak.ClientID,
			cfg.Keycloak.ClientSecret, hostURI, cfg.WebURL,
		)
		if err != nil {
			return nil, fmt.Errorf("could not initialise the OAuth provider, err: %w", err)
		}
	}

	r := gin.New()
	r.Use(Logger(log), gin.Recovery())
	r.Use(ErrorHandle())
	r.Use(CORSMiddleware())
	r.NoRoute(errors.Return404)

	a := &API{r: r, db: database, log: log, oa2Prov: oa2Prov}
	a.InitRoutes()
	return a, nil
}

func (a *API) Router() *gin.Engine {
	return a.r
}
