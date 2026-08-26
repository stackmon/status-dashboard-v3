package notification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func TestKindForStatus(t *testing.T) {
	//nolint:exhaustive // duplicate string constants for Info/Maintenance
	cases := map[event.Status]string{
		event.MaintenancePendingReview: db.NotificationKindPendingReview,
		event.MaintenanceReviewed:      db.NotificationKindReviewed,
		event.MaintenancePlanned:       db.NotificationKindStatusChanged,
		event.MaintenanceInProgress:    db.NotificationKindStatusChanged,
		event.MaintenanceCompleted:     db.NotificationKindStatusChanged,
		event.MaintenanceCancelled:     db.NotificationKindStatusChanged,
	}
	for status, want := range cases {
		assert.Equal(t, want, KindForStatus(status), "status %s", status)
	}
}

func TestBackoff_ProgressionAndCap(t *testing.T) {
	base := 5 * time.Minute
	fn := Backoff(base)

	before := time.Now().UTC()
	// attempt 1 -> ~5m, attempt 2 -> ~10m, attempt 3 -> ~20m.
	assert.WithinDuration(t, before.Add(5*time.Minute), fn(1), time.Second)
	assert.WithinDuration(t, before.Add(10*time.Minute), fn(2), time.Second)
	assert.WithinDuration(t, before.Add(20*time.Minute), fn(3), time.Second)

	// Large attempt count is capped at maxBackoff (2h).
	assert.WithinDuration(t, before.Add(maxBackoff), fn(20), time.Second)
}

func TestConfigFromConf_Disabled(t *testing.T) {
	c := &conf.Config{}
	cfg, err := ConfigFromConf(c)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
}

func TestConfigFromConf_ParsesEnabled(t *testing.T) {
	c := &conf.Config{
		WebURL: "https://status.example.com/",
		SMTP: conf.SMTPConfig{
			Host: "smtp.otc", Port: "587", From: "sd@com.com",
			User: "u", Password: "p", TLS: true, Timeout: "30s",
		},
		Notifications: conf.NotificationsConfig{
			Enabled: true, LeaseTimeout: "60s", MaxAttempts: "5", BackoffInterval: "5m",
			SmodEmail: "support@com.com", EmailsOperators: "ops1@com.com, ops2@com.com",
			EmailsAdmins: "admin@com.com",
		},
	}

	cfg, err := ConfigFromConf(c)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 587, cfg.Port)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 60*time.Second, cfg.LeaseTimeout)
	assert.Equal(t, 5, cfg.MaxAttempts)
	assert.Equal(t, 5*time.Minute, cfg.BackoffBase)
	assert.Equal(t, []string{"ops1@com.com", "ops2@com.com"}, cfg.ReviewOperators)
	assert.Equal(t, "https://status.example.com", cfg.BaseURL, "trailing slash trimmed")
}
