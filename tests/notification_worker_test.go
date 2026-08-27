package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/notification"
)

// fakeSender records deliveries and can fail or panic for chosen recipients.
type fakeSender struct {
	mu       sync.Mutex
	sent     []string
	failFor  map[string]bool
	panicFor map[string]bool
}

func (f *fakeSender) Send(_ context.Context, recipient string, _ notification.Email) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicFor[recipient] {
		panic("smtp exploded for " + recipient)
	}
	if f.failFor[recipient] {
		return errors.New("smtp failed for " + recipient)
	}
	f.sent = append(f.sent, recipient)
	return nil
}

func (f *fakeSender) recipients() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

func testWorker(t *testing.T, d *db.DB, sender notification.Sender, maxAttempts int) *notification.Worker {
	t.Helper()
	w, err := notification.NewWorker(notification.Config{
		Enabled:      true,
		LeaseTimeout: time.Minute,
		MaxAttempts:  maxAttempts,
		BackoffBase:  5 * time.Minute,
		Timeout:      30 * time.Second,
	}, d, sender, zap.NewNop(), nil)
	require.NoError(t, err)
	return w
}

func fetchByDedup(t *testing.T, g *gorm.DB, dedup string) db.NotificationOutbox {
	t.Helper()
	var row db.NotificationOutbox
	require.NoError(t, g.Where("dedup_key = ?", dedup).First(&row).Error)
	return row
}

func TestWorker_DeliversAllPending(t *testing.T) {
	truncateIncidents(t)
	ctx := context.Background()
	d, g := newNotifDB(t)
	incID := seedIncident(t, d)

	recipients := []string{"a@com.com", "b@com.com", "c@com.com"}
	for _, rc := range recipients {
		require.NoError(t, d.Enqueue(ctx, nil, newOutboxRow(incID, rc)))
	}

	fake := &fakeSender{}
	require.NoError(t, testWorker(t, d, fake, 3).Drain(ctx))

	assert.ElementsMatch(t, recipients, fake.recipients())

	var notSent int64
	require.NoError(t, g.Model(&db.NotificationOutbox{}).
		Where("incident_id = ? AND status <> ?", incID, db.NotificationStatusSent).
		Count(&notSent).Error)
	assert.Equal(t, int64(0), notSent, "all rows delivered")
}

func TestWorker_FailedSendRetriesWithBackoff(t *testing.T) {
	truncateIncidents(t)
	ctx := context.Background()
	d, g := newNotifDB(t)
	incID := seedIncident(t, d)

	good := newOutboxRow(incID, "good@com.com")
	bad := newOutboxRow(incID, "bad@com.com")
	require.NoError(t, d.Enqueue(ctx, nil, good))
	require.NoError(t, d.Enqueue(ctx, nil, bad))

	fake := &fakeSender{failFor: map[string]bool{"bad@com.com": true}}
	require.NoError(t, testWorker(t, d, fake, 3).Drain(ctx)) // retries remain

	assert.Equal(t, []string{"good@com.com"}, fake.recipients())

	badRow := fetchByDedup(t, g, bad.DedupKey)
	assert.Equal(t, db.NotificationStatusPending, badRow.Status)
	assert.Equal(t, 1, badRow.Attempts)
	require.NotNil(t, badRow.NextAttemptAt)
	assert.True(t, badRow.NextAttemptAt.After(time.Now().UTC()), "backoff pushed next_attempt_at forward")
	require.NotNil(t, badRow.LastError)

	assert.Equal(t, db.NotificationStatusSent, fetchByDedup(t, g, good.DedupKey).Status)
}

func TestWorker_FailedSendMarksFailedAtMaxAttempts(t *testing.T) {
	truncateIncidents(t)
	ctx := context.Background()
	d, g := newNotifDB(t)
	incID := seedIncident(t, d)

	bad := newOutboxRow(incID, "bad@com.com")
	require.NoError(t, d.Enqueue(ctx, nil, bad))

	fake := &fakeSender{failFor: map[string]bool{"bad@com.com": true}}
	require.NoError(t, testWorker(t, d, fake, 1).Drain(ctx)) // no retries left after first attempt

	badRow := fetchByDedup(t, g, bad.DedupKey)
	assert.Equal(t, db.NotificationStatusFailed, badRow.Status)
	assert.Nil(t, badRow.NextAttemptAt)
	require.NotNil(t, badRow.LastError)
}

func TestWorker_PanicIsolatedPerRow(t *testing.T) {
	truncateIncidents(t)
	ctx := context.Background()
	d, g := newNotifDB(t)
	incID := seedIncident(t, d)

	good := newOutboxRow(incID, "good@com.com")
	boom := newOutboxRow(incID, "boom@com.com")
	require.NoError(t, d.Enqueue(ctx, nil, good))
	require.NoError(t, d.Enqueue(ctx, nil, boom))

	fake := &fakeSender{panicFor: map[string]bool{"boom@com.com": true}}
	// Drain must not propagate the panic.
	require.NoError(t, testWorker(t, d, fake, 3).Drain(ctx))

	assert.Equal(t, []string{"good@com.com"}, fake.recipients())

	boomRow := fetchByDedup(t, g, boom.DedupKey)
	assert.Equal(t, db.NotificationStatusPending, boomRow.Status)
	require.NotNil(t, boomRow.LastError)
	assert.Contains(t, *boomRow.LastError, "panic")
}

func TestWorker_DrainTwiceDoesNotResend(t *testing.T) {
	truncateIncidents(t)
	ctx := context.Background()
	d, _ := newNotifDB(t)
	incID := seedIncident(t, d)
	require.NoError(t, d.Enqueue(ctx, nil, newOutboxRow(incID, "a@com.com")))

	fake := &fakeSender{}
	w := testWorker(t, d, fake, 3)
	require.NoError(t, w.Drain(ctx))
	require.NoError(t, w.Drain(ctx)) // sent rows must not be re-claimed

	assert.Equal(t, []string{"a@com.com"}, fake.recipients())
}
