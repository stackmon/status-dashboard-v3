package tests

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/api"
	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const (
	pgImage   = "postgres:15-alpine"
	pgDump    = "dump_test.sql"
	pgDumpDir = "testdata"

	dbName     = "status_dashboard"
	dbUser     = "pg"
	dbPassword = "pass"
)

var databaseURL = "postgresql://%s:%s@localhost:%s/%s"

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := postgres.Run(ctx,
		pgImage,
		postgres.WithInitScripts(filepath.Join(pgDumpDir, pgDump)),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	defer func() {
		if err = testcontainers.TerminateContainer(container); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("failed to start container: %s", err)
		return
	}

	ports, _ := container.Ports(ctx)
	port := ports["5432/tcp"][0].HostPort
	databaseURL = fmt.Sprintf(databaseURL, dbUser, dbPassword, port, dbName)

	m.Run()
}

func initTests(t *testing.T) (*gin.Engine, *db.DB, *auth.Provider) {
	t.Helper()
	t.Log("init structs")

	d, err := db.New(&conf.Config{
		DB: databaseURL,
		// if you want to debug gorm, uncomment it
		//LogLevel: conf.DevelopMode,
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.NoRoute(errors.Return404)
	r.Use(api.ErrorHandle())

	logger, _ := zap.NewDevelopment()

	cfg, err := conf.LoadConf()
	require.NoError(t, err)

	oa2Prov, err := auth.NewProvider(cfg.Keycloak.URL, cfg.Keycloak.Realm, cfg.Keycloak.ClientID, cfg.Keycloak.ClientSecret, cfg.Hostname, cfg.WebURL)
	require.NoError(t, err)

	initRoutesAuth(t, r, oa2Prov, logger)
	initRoutesV1(t, r, d, logger)
	initRoutesV2(t, r, d, logger)

	return r, d, oa2Prov
}

func initRoutesAuth(t *testing.T, c *gin.Engine, oa2Prov *auth.Provider, logger *zap.Logger) {
	t.Helper()
	t.Log("init routes for auth")

	authAPI := c.Group("auth")

	authAPI.GET("login", auth.GetLoginPageHandler(oa2Prov, logger))
	authAPI.GET("callback", auth.GetCallbackHandler(oa2Prov, logger))
	authAPI.POST("token", auth.PostTokenHandler(oa2Prov, logger))
	authAPI.POST("logout", auth.PostTokenHandler(oa2Prov, logger))
}

func initRoutesV1(t *testing.T, c *gin.Engine, dbInst *db.DB, logger *zap.Logger) {
	t.Helper()
	t.Log("init routes for V1")

	v1Api := c.Group("v1")

	v1Api.GET("component_status", v1.GetComponentsStatusHandler(dbInst, logger))
	v1Api.POST("component_status", v1.PostComponentStatusHandler(dbInst, logger))

	v1Api.GET("incidents", v1.GetIncidentsHandler(dbInst, logger))
}

func initRoutesV2(t *testing.T, c *gin.Engine, dbInst *db.DB, logger *zap.Logger) {
	t.Helper()
	t.Log("init routes for V2")

	v2Api := c.Group("v2")

	v2Api.GET("components", v2.GetComponentsHandler(dbInst, logger))
	v2Api.POST("components", v2.PostComponentHandler(dbInst, logger))
	v2Api.GET("components/:id", v2.GetComponentHandler(dbInst, logger))

	v2Api.GET("incidents", v2.GetIncidentsHandler(dbInst, logger))
	v2Api.POST("incidents", api.ValidateComponentsMW(dbInst, logger), v2.PostIncidentHandler(dbInst, logger))
	v2Api.GET("incidents/:id", v2.GetIncidentHandler(dbInst, logger))
	v2Api.PATCH("incidents/:id", v2.PatchIncidentHandler(dbInst, logger))
	v2Api.POST("incidents/:id/extract", v2.PostIncidentExtractHandler(dbInst, logger))
	v2Api.PATCH("incidents/:id/updates/:update_id", v2.PatchEventUpdateTextHandler(dbInst, logger))

	v2Api.GET("availability", v2.GetComponentsAvailabilityHandler(dbInst, logger))
}

func truncateIncidents(t *testing.T) {
	t.Helper()
	t.Log("cleaning up incident-related tables before test")

	// Create a new GORM DB connection just for truncation.
	// This avoids modifying the db package to expose the underlying *gorm.DB.
	gormDB, err := gorm.Open(gormpostgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err, "failed to open gorm connection for truncation")

	result := gormDB.Exec("TRUNCATE TABLE incident, incident_status, incident_component_relation RESTART IDENTITY")
	require.NoError(t, result.Error, "failed to truncate incident tables")

	sqlDB, err := gormDB.DB()
	require.NoError(t, err, "failed to get sql.DB from gorm for closing")
	err = sqlDB.Close()
	require.NoError(t, err, "failed to close gorm connection for truncation")
}
