package v2

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/notification"
)

// statsStaleThreshold marks a processing row as stuck when its lease is older than
// this — comfortably beyond the default lease timeout so live sends are not flagged.
const statsStaleThreshold = 2 * time.Minute

// defaultFailedListLimit bounds the failed-rows listing.
const defaultFailedListLimit = 100

// requireAdmin ensures the caller resolved to the Admin role. It writes the error
// response and returns false when not.
func requireAdmin(c *gin.Context, logger *zap.Logger) bool {
	role, ok := getRoleFromContext(c, logger)
	if !ok {
		return false
	}
	if role != rbac.Admin {
		apiErrors.RaiseForbiddenErr(c, apiErrors.ErrAuthForbidden)
		return false
	}
	return true
}

// GetNotificationStatsHandler returns the outbox queue statistics (admin only).
func GetNotificationStatsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c, logger) {
			return
		}

		stats, err := dbInst.GetNotificationStats(c.Request.Context(), statsStaleThreshold)
		if err != nil {
			logger.Error("failed to get notification stats", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// GetFailedNotificationsHandler lists the most recent failed rows (admin only).
func GetFailedNotificationsHandler(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireAdmin(c, logger) {
			return
		}

		rows, err := dbInst.ListFailedNotifications(c.Request.Context(), defaultFailedListLimit)
		if err != nil {
			logger.Error("failed to list failed notifications", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}
}

// RedriveNotificationsData is the optional re-drive request body.
type RedriveNotificationsData struct {
	// IDs limits the re-drive to specific outbox rows; empty means all failed rows.
	IDs []uint `json:"ids"`
}

// RedriveNotificationsHandler resets failed rows back to pending (admin only) and
// wakes the worker to retry them immediately.
func RedriveNotificationsHandler(dbInst *db.DB, logger *zap.Logger, pub ...*notification.Publisher) gin.HandlerFunc {
	publisher := optionalPublisher(pub)
	return func(c *gin.Context) {
		if !requireAdmin(c, logger) {
			return
		}

		var body RedriveNotificationsData
		// A missing/empty body is valid: re-drive everything.
		_ = c.ShouldBindBodyWithJSON(&body)

		count, err := dbInst.RedriveFailed(c.Request.Context(), body.IDs...)
		if err != nil {
			logger.Error("failed to re-drive notifications", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		if count > 0 {
			publisher.Notify() // wake the worker to pick up the re-driven rows
		}
		logger.Info("re-drove failed notifications", zap.Int64("count", count))
		c.JSON(http.StatusOK, gin.H{"redriven": count})
	}
}
