package api

import (
	"context"
	"testing"
	"time"

	"imagestudio/internal/config"
)

func TestRedisJobQueueKeyUsesStoragePrefix(t *testing.T) {
	if got := redisJobQueueKey("test:prefix:"); got != "test:prefix:business_jobs:queue" {
		t.Fatalf("redisJobQueueKey() = %q", got)
	}
}

func TestNewBusinessJobDispatcherDefaultsToLocal(t *testing.T) {
	cfg := &config.Config{}
	cfg.JobQueue.Backend = "local"

	dispatcher, err := newBusinessJobDispatcher(cfg)
	if err != nil {
		t.Fatalf("newBusinessJobDispatcher() returned error: %v", err)
	}
	defer dispatcher.Close()
	if _, ok := dispatcher.(*localBusinessJobDispatcher); !ok {
		t.Fatalf("dispatcher type = %T, want localBusinessJobDispatcher", dispatcher)
	}
}

func TestLocalBusinessJobDispatcherKeepsFullNotification(t *testing.T) {
	dispatcher := newLocalBusinessJobDispatcher()
	defer dispatcher.Close()

	err := dispatcher.Notify(context.Background(), businessJobNotification{
		JobID:  "job-local",
		UserID: "user-local",
	})
	if err != nil {
		t.Fatalf("Notify() returned error: %v", err)
	}

	select {
	case notification := <-dispatcher.Notifications():
		if notification.JobID != "job-local" || notification.UserID != "user-local" {
			t.Fatalf("notification = %#v", notification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for local notification")
	}
}
