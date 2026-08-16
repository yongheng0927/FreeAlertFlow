package service

import (
	"context"
	"testing"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

func TestRetentionCleanerDeletesExpiredAlerts(t *testing.T) {
	alerts := newFakeAlertStore()
	old := &model.Alert{ReceivedAt: time.Now().Add(-48 * time.Hour)}
	recent := &model.Alert{ReceivedAt: time.Now()}
	if err := alerts.Create(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if err := alerts.Create(context.Background(), recent); err != nil {
		t.Fatal(err)
	}

	c := NewRetentionCleaner(alerts, 24*time.Hour)
	if err := c.cleanOnce(context.Background()); err != nil {
		t.Fatalf("cleanOnce: %v", err)
	}
	if got, _ := alerts.FindByID(context.Background(), old.ID); got != nil {
		t.Error("expired alert must be deleted")
	}
	if got, _ := alerts.FindByID(context.Background(), recent.ID); got == nil {
		t.Error("recent alert must be kept")
	}
}

func TestRetentionCleanerDisabled(t *testing.T) {
	c := NewRetentionCleaner(newFakeAlertStore(), 0)
	// retention <= 0 时 Run 直接返回，不启动任何清理
	done := make(chan struct{})
	go func() {
		c.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run must return immediately when retention is disabled")
	}
}
