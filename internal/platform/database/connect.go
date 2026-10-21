package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"
)

func ConnectionTimeout() (time.Duration, error) {
	value := os.Getenv("DB_CONNECT_TIMEOUT")
	if value == "" {
		return 180 * time.Second, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("DB_CONNECT_TIMEOUT must be a positive duration")
	}
	return timeout, nil
}

func WaitForConnection(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	return waitForConnection(ctx, db.PingContext, log, time.Second, 5*time.Second)
}

func waitForConnection(ctx context.Context, ping func(context.Context) error, log *slog.Logger, interval, attemptTimeout time.Duration) error {
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("database connection wait ended: %w (last error: %v)", err, lastErr)
		}
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err := ping(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		log.WarnContext(ctx, "waiting for database connection", "attempt", attempt, "error", err.Error())
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("database connection wait ended: %w (last error: %v)", ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}
