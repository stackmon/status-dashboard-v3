package main

import (
	"context"
	"errors"
	"github.com/stackmon/otc-status-dashboard/internal/app"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os/signal"
	"syscall"
)

func main() {
	c, err := conf.LoadConf()
	if err != nil {
		log.Fatalf("failed to parse configuration: %s", err.Error())
	}

	logger := conf.NewLogger(c.LogLevel)
	logger.Info("app starting")

	s, err := app.New(c, logger)
	if err != nil {
		logger.Fatal("fail to init app", zap.Error(err))
	}

	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer done()

	go func() {
		if err := s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("app is failed to run", zap.Error(err))
		}
	}()

	<-ctx.Done()
	s.Log.Info("shutdown app")

	if err = s.Shutdown(ctx); err != nil {
		logger.Fatal("app shutdown failed", zap.Error(err))
	}

	logger.Info("app exited")
}
