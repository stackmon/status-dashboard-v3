package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/app"
	"github.com/stackmon/otc-status-dashboard/internal/checker"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
)

func main() {
	c, err := conf.LoadConf()
	if err != nil {
		log.Fatalf("failed to parse configuration: %s", err.Error())
	}

	logger := conf.NewLogger(c.LogLevel)
	logConfig(logger, c)

	s, err := app.New(c, logger)
	if err != nil {
		logger.Fatal("fail to init app", zap.Error(err))
	}

	ch, err := checker.New(c, logger)
	if err != nil {
		logger.Error("fail to init checker", zap.Error(err))
	}
	stopCh := make(chan struct{})

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer done()

	go func() {
		if err = s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("app is failed to run", zap.Error(err))
		}
	}()

	go func() {
		ch.Run(stopCh)
	}()

	<-ctx.Done()
	s.Log.Info("shutdown app")

	if err = s.Shutdown(ctx); err != nil {
		logger.Fatal("app shutdown failed", zap.Error(err))
	}

	if err = ch.Shutdown(stopCh); err != nil {
		logger.Fatal("checker shutdown failed", zap.Error(err))
	}

	logger.Info("app exited")
}

func logConfig(logger *zap.Logger, c *conf.Config) {
	logger.Info("app starting", zap.String("log_level", c.LogLevel))

	logger.Info("checking configuration parameters")
	if c.DB != "" {
		logger.Info("database connection string is set")
	} else {
		logger.Warn("database connection string is not set")
	}

	logger.Info("authentication status", zap.Bool("disabled", c.AuthenticationDisabled))
	if !c.AuthenticationDisabled && c.AuthGroup == "" {
		logger.Warn("Auth group is not set, which may cause authorization issues.")
	}

	logger.Debug("application endpoint configuration",
		zap.String("hostname", c.Hostname),
		zap.String("port", c.Port),
		zap.String("web_url", c.WebURL))
}
