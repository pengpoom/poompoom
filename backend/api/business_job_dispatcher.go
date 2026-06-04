package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"imagestudio/internal/businessjobs"
	"imagestudio/internal/config"

	"github.com/redis/go-redis/v9"
)

type businessJobNotification struct {
	JobID  string
	UserID string
}

type businessJobDispatcher interface {
	Notify(ctx context.Context, notification businessJobNotification) error
	Notifications() <-chan businessJobNotification
	WorkerID() string
	Close() error
}

type localBusinessJobDispatcher struct {
	workerID      string
	notifications chan businessJobNotification
}

func newLocalBusinessJobDispatcher() businessJobDispatcher {
	return &localBusinessJobDispatcher{
		workerID:      fmt.Sprintf("local-%s", businessjobs.NewJobID()),
		notifications: make(chan businessJobNotification, 128),
	}
}

func newBusinessJobDispatcher(cfg *config.Config) (businessJobDispatcher, error) {
	if cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.JobQueue.Backend), "redis") {
		return newRedisBusinessJobDispatcher(cfg)
	}
	return newLocalBusinessJobDispatcher(), nil
}

func (d *localBusinessJobDispatcher) Notify(ctx context.Context, notification businessJobNotification) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case d.notifications <- notification:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (d *localBusinessJobDispatcher) Notifications() <-chan businessJobNotification {
	return d.notifications
}

func (d *localBusinessJobDispatcher) WorkerID() string {
	if d == nil || d.workerID == "" {
		return fmt.Sprintf("local-%d", time.Now().UnixNano())
	}
	return d.workerID
}

func (d *localBusinessJobDispatcher) Close() error {
	return nil
}

type redisBusinessJobDispatcher struct {
	workerID      string
	queueKey      string
	client        *redis.Client
	notifications chan businessJobNotification
	stop          chan struct{}
	done          chan struct{}
}

func newRedisBusinessJobDispatcher(cfg *config.Config) (businessJobDispatcher, error) {
	addr := strings.TrimSpace(cfg.Storage.RedisAddr)
	if addr == "" {
		return nil, fmt.Errorf("redis addr is required when job_queue.backend=redis")
	}
	dispatcher := &redisBusinessJobDispatcher{
		workerID:      fmt.Sprintf("redis-%s", businessjobs.NewJobID()),
		queueKey:      redisJobQueueKey(cfg.Storage.RedisPrefix),
		client:        redis.NewClient(&redis.Options{Addr: addr, Password: cfg.Storage.RedisPassword, DB: cfg.Storage.RedisDB}),
		notifications: make(chan businessJobNotification, 128),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go dispatcher.loop()
	return dispatcher, nil
}

func redisJobQueueKey(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "imagestudio:studio"
	}
	return strings.TrimRight(prefix, ":") + ":business_jobs:queue"
}

func (d *redisBusinessJobDispatcher) Notify(ctx context.Context, notification businessJobNotification) error {
	if d == nil || d.client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	jobID := cleanJobNotificationID(notification.JobID)
	if jobID == "" {
		return d.notifyLocal(ctx, businessJobNotification{})
	}
	if err := d.client.RPush(ctx, d.queueKey, jobID).Err(); err != nil {
		_ = d.notifyLocal(context.Background(), businessJobNotification{})
		return err
	}
	return nil
}

func (d *redisBusinessJobDispatcher) Notifications() <-chan businessJobNotification {
	return d.notifications
}

func (d *redisBusinessJobDispatcher) WorkerID() string {
	if d == nil || d.workerID == "" {
		return fmt.Sprintf("redis-%d", time.Now().UnixNano())
	}
	return d.workerID
}

func (d *redisBusinessJobDispatcher) Close() error {
	if d == nil {
		return nil
	}
	select {
	case <-d.done:
	default:
		close(d.stop)
		<-d.done
	}
	if d.client == nil {
		return nil
	}
	return d.client.Close()
}

func (d *redisBusinessJobDispatcher) loop() {
	defer close(d.done)
	for {
		select {
		case <-d.stop:
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result, err := d.client.BLPop(ctx, time.Second, d.queueKey).Result()
		cancel()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			_ = d.notifyLocal(context.Background(), businessJobNotification{})
			time.Sleep(time.Second)
			continue
		}
		if len(result) < 2 {
			continue
		}
		_ = d.notifyLocal(context.Background(), businessJobNotification{JobID: cleanJobNotificationID(result[1])})
	}
}

func (d *redisBusinessJobDispatcher) notifyLocal(ctx context.Context, notification businessJobNotification) error {
	select {
	case d.notifications <- notification:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
