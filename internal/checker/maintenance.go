//nolint:dupl
package checker

import (
	"slices"
	"time"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

type MntStatusHistory struct {
	hasPlanned    bool
	hasInProgress bool
	hasCompleted  bool
	hasCancelled  bool
}

func (st *MntStatusHistory) hasStatus(status event.Status) bool {
	switch status { //nolint:exhaustive
	case event.MaintenancePlanned:
		return st.hasPlanned
	case event.MaintenanceInProgress:
		return st.hasInProgress
	case event.MaintenanceCompleted:
		return st.hasCompleted
	case event.MaintenanceCancelled:
		return st.hasCancelled
	}
	return false
}

func (st *MntStatusHistory) setStatus(status event.Status) {
	switch status { //nolint:exhaustive
	case event.MaintenancePlanned:
		st.hasPlanned = true
	case event.MaintenanceInProgress:
		st.hasInProgress = true
	case event.MaintenanceCompleted:
		st.hasCompleted = true
	case event.MaintenanceCancelled:
		st.hasCancelled = true
	}
}

func (ch *Checker) CheckMaintenance() error {
	ch.log.Info("check maintenances statuses")
	if ch.lastMntID == 0 {
		ch.log.Info("no last completed maintenance, starting from the beginning")
	}

	maintenances, err := ch.db.GetMaintenances(ch.lastMntID)
	if err != nil {
		return err
	}

	var activeMaintenances []uint
	for _, mn := range maintenances {
		sHistory := calculateMntStatusHistory(mn)
		actualStatus := calculateCurrentMntStatus(sHistory, mn)

		switch actualStatus { //nolint:exhaustive
		case event.MaintenancePlanned:
			ch.fixMntMissedStatuses(event.MaintenancePlanned, sHistory, mn)
			activeMaintenances = append(activeMaintenances, mn.ID)
		case event.MaintenanceInProgress:
			ch.fixMntMissedStatuses(event.MaintenanceInProgress, sHistory, mn)
			activeMaintenances = append(activeMaintenances, mn.ID)
		case event.MaintenanceCompleted:
			ch.fixMntMissedStatuses(event.MaintenanceCompleted, sHistory, mn)
		case event.MaintenanceCancelled:
			ch.fixMntMissedStatuses(event.MaintenanceCancelled, sHistory, mn)
		}

		err = ch.db.ModifyIncident(mn)
		if err != nil {
			return err
		}
	}

	if len(activeMaintenances) == 0 {
		for _, mn := range maintenances {
			if mn.ID > ch.lastMntID {
				ch.lastMntID = mn.ID
			}
		}
		ch.log.Debug(
			"there are no actual maintenances, set the last ID to the last one",
			zap.Uint("lastMntID", ch.lastMntID),
		)
	} else {
		ch.lastMntID = slices.Min(activeMaintenances)
		ch.log.Debug(
			"set the last ID to the earliest planned or in progress maintenance",
			zap.Uint("lastMntID", ch.lastMntID),
		)
	}

	ch.log.Info("finished checking maintenances")

	return nil
}

func calculateMntStatusHistory(mn *db.Incident) *MntStatusHistory {
	sHistory := &MntStatusHistory{}
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

func calculateCurrentMntStatus(sHistory *MntStatusHistory, mn *db.Incident) event.Status {
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

func (ch *Checker) fixMntMissedStatuses(status event.Status, sHistory *MntStatusHistory, mnt *db.Incident) {
	ch.log.Info(
		"start to fix missed statuses for the maintenance",
		zap.String("targetStatus", string(status)), zap.Uint("mntID", mnt.ID),
	)

	var statusText string
	var statusTimestamp time.Time

	switch status { //nolint:exhaustive
	case event.MaintenancePlanned:
		ch.log.Info("fixing the planned status for the maintenance", zap.Uint("mntID", mnt.ID))
		if sHistory.hasStatus(status) {
			ch.log.Info("the maintenance is already has planned status", zap.Uint("mntID", mnt.ID))
			return
		}
		statusText = event.MaintenancePlannedStatusText()
		statusTimestamp = *mnt.StartDate

	case event.MaintenanceInProgress:
		ch.log.Info("fixing the active status for the maintenance", zap.Uint("mntID", mnt.ID))
		ch.fixMntMissedStatuses(event.MaintenancePlanned, sHistory, mnt)
		if sHistory.hasStatus(status) {
			ch.log.Info("the maintenance is already has active status", zap.Uint("mntID", mnt.ID))
			return
		}
		statusText = event.MaintenanceInProgressStatusText()
		statusTimestamp = *mnt.StartDate

	case event.MaintenanceCompleted:
		ch.log.Info("fixing the completed status for the maintenance", zap.Uint("mntID", mnt.ID))
		ch.fixMntMissedStatuses(event.MaintenanceInProgress, sHistory, mnt)
		if sHistory.hasStatus(status) {
			ch.log.Info("the maintenance is already has completed status", zap.Uint("mntID", mnt.ID))
			return
		}
		statusText = event.MaintenanceCompletedStatusText()
		statusTimestamp = *mnt.EndDate
	case event.MaintenanceCancelled:
		ch.log.Info("fixing the cancelled status for the maintenance", zap.Uint("mntID", mnt.ID))
		ch.fixMntMissedStatuses(event.MaintenancePlanned, sHistory, mnt)
		ch.log.Info("the maintenance is already has cancelled status", zap.Uint("mntID", mnt.ID))
		return
	}

	mnt.Statuses = append(mnt.Statuses, db.IncidentStatus{
		IncidentID: mnt.ID,
		Status:     status,
		Text:       statusText,
		Timestamp:  statusTimestamp,
	})
	sHistory.setStatus(status)
	ch.log.Info("the status was added", zap.String("status", string(status)), zap.Uint("mntID", mnt.ID))
}
