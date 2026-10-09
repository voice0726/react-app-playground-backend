package database

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/config"
)

func connectionTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestWaitForConnectionRecoversAfterInitialFailures(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitForConnection(ctx, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("connection unavailable")
		}
		return nil
	}, connectionTestLogger(), 0, time.Second)
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}

func TestWaitForConnectionStopsAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := waitForConnection(ctx, func(attemptCtx context.Context) error {
		<-attemptCtx.Done()
		return attemptCtx.Err()
	}, connectionTestLogger(), 0, 5*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
}

func TestWaitForConnectionDoesNotPingAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForConnection(ctx, func(context.Context) error {
		t.Fatal("canceled connection wait attempted a ping")
		return nil
	}, connectionTestLogger(), 0, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestConnectionTimeout(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 180 * time.Second},
		{"3s", 3 * time.Second},
		{"0s", 0},
		{"-1s", 0},
		{"invalid", 0},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("DB_CONNECT_TIMEOUT", tc.value)
			got, err := ConnectionTimeout()
			if got != tc.want || (err != nil) != (tc.want == 0) {
				t.Fatalf("timeout=%v error=%v", got, err)
			}
		})
	}
}

func TestNewDefersConnectionUntilExplicitWait(t *testing.T) {
	db, err := New(config.Config{DatabaseURL: "postgres://playground:playground@127.0.0.1:1/playground?sslmode=disable"}, connectionTestLogger())
	if err != nil {
		t.Fatalf("initialization attempted a connection: %v", err)
	}
	t.Cleanup(func() { _ = Close(db) })
}
