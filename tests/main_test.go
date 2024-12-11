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

	"github.com/stackmon/otc-status-dashboard/internal/api"
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

func initTests(t *testing.T) (*gin.Engine, *db.DB) {
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
	initRoutesV1(t, r, d, logger)
	initRoutesV2(t, r, d, logger)

	return r, d
}

func initRoutesV1(t *testing.T, c *gin.Engine, dbInst *db.DB, log *zap.Logger) {
	t.Helper()
	t.Log("init routes for V1")

	v1Api := c.Group("v1")

	v1Api.GET("component_status", v1.GetComponentsStatusHandler(dbInst, log))
	v1Api.POST("component_status", v1.PostComponentStatusHandler(dbInst, log))

	v1Api.GET("incidents", v1.GetIncidentsHandler(dbInst, log))
}

func initRoutesV2(t *testing.T, c *gin.Engine, dbInst *db.DB, log *zap.Logger) {
	t.Helper()
	t.Log("init routes for V2")

	v2Api := c.Group("v2")

	v2Api.GET("components", v2.GetComponentsHandler(dbInst, log))
	v2Api.POST("components", v2.PostComponentHandler(dbInst, log))
	v2Api.GET("components/:id", v2.GetComponentHandler(dbInst, log))

	v2Api.GET("incidents", v2.GetIncidentsHandler(dbInst, log))
	v2Api.POST("incidents", api.ValidateComponentsMW(dbInst, log), v2.PostIncidentHandler(dbInst, log))
	v2Api.GET("incidents/:id", v2.GetIncidentHandler(dbInst, log))
	v2Api.PATCH("incidents/:id", api.ValidateComponentsMW(dbInst, log), v2.PatchIncidentHandler(dbInst, log))
}
