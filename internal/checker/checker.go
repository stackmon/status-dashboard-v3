package checker

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const defaultPeriod = time.Minute * 5

type Checker struct {
	db  *db.DB
	log *zap.Logger
	// lastIDs are the earliest planned or in progress maintenance/info events ID.
	lastMntID  uint
	lastInfoID uint
}

func New(c *conf.Config, log *zap.Logger) (*Checker, error) {
	dbNew, err := db.New(c)
	if err != nil {
		return nil, err
	}
	return &Checker{db: dbNew, log: log}, nil
}

func (ch *Checker) Check() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		err := ch.CheckMaintenance()
		if err != nil {
			ch.log.Error("error to check maintenances", zap.Error(err))
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		err := ch.CheckInfoEvents()
		if err != nil {
			ch.log.Error("error to check info events", zap.Error(err))
		}
		wg.Done()
	}()

	wg.Wait()
}

func (ch *Checker) Run(done chan struct{}) {
	ch.log.Info("checker is started")
	ticker := time.NewTicker(defaultPeriod)
	defer ticker.Stop()

	for { //nolint:nolintlint
		select {
		case <-done:
			return
		case <-ticker.C:
			ch.Check()
		}
	}
}

func (ch *Checker) Shutdown(done chan struct{}) error {
	ch.log.Info("start to shutdown checker")
	done <- struct{}{}
	close(done)
	return ch.db.Close()
}
