package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
)

func TestWaitForShutdownReturnsServerStartError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()

	cfg := config.Config{ServerAddr: listener.Addr().String()}
	server := httpserver.New(cfg, nil, slog.Default(), authgateway.NewClient(cfg))
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start(context.Background())
	}()

	startErr := waitForShutdown(make(chan os.Signal), serverErrors)
	var opErr *net.OpError
	if !errors.As(startErr, &opErr) {
		t.Fatalf("waitForShutdown() error = %v, want a wrapped net.OpError", startErr)
	}
}

func TestWaitForShutdownAcceptsSIGTERM(t *testing.T) {
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM

	if err := waitForShutdown(signals, make(chan error)); err != nil {
		t.Fatalf("waitForShutdown() error = %v, want nil", err)
	}
}

func TestWaitForShutdownPrioritizesReadyServerErrorOverSIGTERM(t *testing.T) {
	startErr := errors.New("listen tcp: bind: address already in use")
	signals := make(chan os.Signal, 1)
	signals <- syscall.SIGTERM
	serverErrors := make(chan error, 1)
	serverErrors <- startErr

	err := waitForShutdown(signals, serverErrors)
	if !errors.Is(err, startErr) {
		t.Fatalf("waitForShutdown() error = %v, want wrapped %v", err, startErr)
	}
}
