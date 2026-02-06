package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

type API struct {
	r           *gin.Engine
	db          *db.DB
	log         *zap.Logger
	oa2Prov     *auth.Provider
	secretKeyV1 string
	rbac        *rbac.Service
}

func New(cfg *conf.Config, log *zap.Logger, database *db.DB) (*API, error) {
	if cfg.LogLevel != conf.DevelopMode {
		gin.SetMode(gin.ReleaseMode)
	}

	oa2Prov := &auth.Provider{Disabled: true}

	if !cfg.AuthenticationDisabled {
		var err error
		if oa2Prov, err = auth.NewProvider(
			cfg.Keycloak.URL, cfg.Keycloak.Realm, cfg.Keycloak.ClientID,
			cfg.Keycloak.ClientSecret, cfg.Hostname, cfg.WebURL,
		); err != nil {
			return nil, fmt.Errorf("could not initialise the OAuth provider, err: %w", err)
		}
	}

	r := gin.New()
	r.Use(Logger(log), gin.Recovery())
	r.Use(ErrorHandle())
	r.Use(CORSMiddleware())
	r.NoRoute(errors.Return404)

	rbacService := rbac.New(cfg.CreatorsGroup, cfg.OperatorsGroup, cfg.AdminsGroup)

	a := &API{
		r:           r,
		db:          database,
		log:         log,
		oa2Prov:     oa2Prov,
		secretKeyV1: cfg.SecretKeyV1,
		rbac:        rbacService,
	}
	a.InitRoutes()
	return a, nil
}

func (a *API) Router() *gin.Engine {
	return a.r
}
