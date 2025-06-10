package checker

import (
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

const defaultPeriod = time.Minute * 5

type Checker struct {
	db  *db.DB
	log *zap.Logger
	// lastID is the earliest planned or in progress maintenance ID.
	lastID uint
}

func New(c *conf.Config, log *zap.Logger) (*Checker, error) {
	dbNew, err := db.New(c)
	if err != nil {
		return nil, err
	}
	return &Checker{db: dbNew, log: log}, nil
}

type StatusHistory struct {
	hasPlanned    bool
	hasInProgress bool
	hasCompleted  bool
	hasCancelled  bool
}

func (ch *Checker) Check() error {
	ch.log.Info("check maintenances statuses")
	if ch.lastID == 0 {
		ch.log.Info("no last completed maintenance, starting from the beginning")
	}

	maintenances, err := ch.db.GetMaintenances(ch.lastID)
	if err != nil {
		return err
	}

	var actualMN []uint
	for _, mn := range maintenances {
		sHistory := calculateStatusHistory(mn)
		actualStatus := calculateCurrentStatus(sHistory, mn)

		// collect statuses for correction
		missedStatuses := make([]db.IncidentStatus, 0)

		switch actualStatus { //nolint:exhaustive
		case event.MaintenancePlanned:
			ch.fixPlannedStatus(sHistory, mn)
			actualMN = append(actualMN, mn.ID)
		case event.MaintenanceInProgress:
			ch.fixInProgressStatus(sHistory, mn)
			actualMN = append(actualMN, mn.ID)
		case event.MaintenanceCompleted:
			ch.fixCompletedStatus(sHistory, mn)
		case event.MaintenanceCancelled:
			ch.fixCancelledStatus(sHistory, mn)
		}

		mn.Statuses = append(mn.Statuses, missedStatuses...)
		err = ch.db.ModifyIncident(mn)
		if err != nil {
			return err
		}
	}

	if len(actualMN) == 0 {
		for _, mn := range maintenances {
			if mn.ID > ch.lastID {
				ch.lastID = mn.ID
			}
		}
		ch.log.Debug("there are no actual maintenances, set the last ID to the last one", zap.Uint("lastID", ch.lastID))
	} else {
		ch.lastID = slices.Min(actualMN)
		ch.log.Debug("set the last ID to the earliest planned or in progress maintenance", zap.Uint("lastID", ch.lastID))
	}

	ch.log.Info("finished checking maintenances")

	return nil
}

func calculateStatusHistory(mn *db.Incident) *StatusHistory {
	sHistory := &StatusHistory{}
	for _, st := range mn.Statuses {
		if st.Status == event.MaintenancePlanned {
			sHistory.hasPlanned = true
		}
		if st.Status == event.MaintenanceInProgress {
			sHistory.hasInProgress = true
		}
		if st.Status == event.MaintenanceCompleted {
			sHistory.hasCompleted = true
		}
		if st.Status == event.MaintenanceCancelled {
			sHistory.hasCancelled = true
		}
	}

	return sHistory
}
func calculateCurrentStatus(sHistory *StatusHistory, mn *db.Incident) event.Status {
	if sHistory.hasCancelled {
		return event.MaintenanceCancelled
	}

	now := time.Now().UTC()

	// calculate the mn current status
	if mn.StartDate.After(now) {
		return event.MaintenancePlanned
	}

	if mn.StartDate.Before(now) && mn.EndDate.After(now) {
		return event.MaintenanceInProgress
	}

	return event.MaintenanceCompleted
}

func (ch *Checker) fixPlannedStatus(sHistory *StatusHistory, mn *db.Incident) {
	if sHistory.hasPlanned {
		ch.log.Debug("the maintenance is already has planned status", zap.Uint("maintenance_id", mn.ID))
		return
	}

	mn.Statuses = append(mn.Statuses, db.IncidentStatus{
		IncidentID: mn.ID,
		Status:     event.MaintenancePlanned,
		Text:       event.MaintenancePlannedDescription(*mn.StartDate, *mn.EndDate),
		Timestamp:  *mn.StartDate,
	})

	sHistory.hasPlanned = true
	ch.log.Info("the status 'planned' was added", zap.Uint("maintenance_id", mn.ID))
}

// fixCancelledStatus checks if the maintenance has planned and in progress statuses.
func (ch *Checker) fixInProgressStatus(sHistory *StatusHistory, mn *db.Incident) {
	ch.fixPlannedStatus(sHistory, mn)

	if sHistory.hasInProgress {
		ch.log.Debug("the maintenance is already has status 'in_progress'", zap.Uint("maintenance_id", mn.ID))
		return
	}

	mn.Statuses = append(mn.Statuses, db.IncidentStatus{
		IncidentID: mn.ID,
		Status:     event.MaintenanceInProgress,
		Text:       "The maintenance is started.",
		Timestamp:  *mn.StartDate,
	})

	sHistory.hasInProgress = true
	ch.log.Info("the status 'in_progress' was added", zap.Uint("maintenance_id", mn.ID))
}

func (ch *Checker) fixCompletedStatus(sHistory *StatusHistory, mn *db.Incident) {
	ch.fixPlannedStatus(sHistory, mn)
	ch.fixInProgressStatus(sHistory, mn)

	if sHistory.hasCompleted {
		ch.log.Debug("the maintenance is already has status 'completed'", zap.Uint("maintenance_id", mn.ID))
		return
	}
	mn.Statuses = append(mn.Statuses, db.IncidentStatus{
		IncidentID: mn.ID,
		Status:     event.MaintenanceCompleted,
		Text:       "The maintenance is completed.",
		Timestamp:  *mn.EndDate,
	})

	ch.log.Info("the status 'completed' was added", zap.Uint("maintenance_id", mn.ID))
}

func (ch *Checker) fixCancelledStatus(sHistory *StatusHistory, mn *db.Incident) {
	ch.fixPlannedStatus(sHistory, mn)
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
			if err := ch.Check(); err != nil {
				ch.log.Error("error to check statuses", zap.Error(err))
			}
		}
	}
}

func (ch *Checker) Shutdown(done chan struct{}) error {
	ch.log.Info("start to shutdown checker")
	done <- struct{}{}
	close(done)
	return ch.db.Close()
}
