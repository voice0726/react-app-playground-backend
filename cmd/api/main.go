package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/dashboard"
	"github.com/voice0726/react-app-playground-backend/internal/deal"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
	"github.com/voice0726/react-app-playground-backend/internal/organization"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/database"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/platform/logging"
	"github.com/voice0726/react-app-playground-backend/internal/task"
	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"go.uber.org/fx"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		log.Print(err)
		return 1
	}
	logger := logging.New(cfg)
	connectionTimeout, err := database.ConnectionTimeout()
	if err != nil {
		logger.Error("invalid database connection timeout", "error", err.Error())
		return 1
	}
	db, err := database.New(cfg, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err.Error())
		return 1
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("database initialization failed", "error", err.Error())
		_ = database.Close(db)
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectCtx, stopConnecting := context.WithTimeout(ctx, connectionTimeout)
	err = database.WaitForConnection(connectCtx, sqlDB, logger)
	stopConnecting()
	if err != nil {
		_ = sqlDB.Close()
		logger.Error("database unavailable", "error", err.Error())
		return 1
	}
	organizationService := organization.NewService(organization.NewRepository(db))
	server := httpserver.New(cfg, db, logger, authgateway.NewClient(cfg), organizationService)
	customerService := customer.NewService(customer.NewRepository(db))
	server.Register(
		organization.NewHandler(organizationService),
		customer.NewHandler(customerService),
		deal.NewHandler(deal.NewService(deal.NewRepository(db), customerService)),
		invoice.NewHandler(invoice.NewService(invoice.NewRepository(db), customerService)),
		task.NewHandler(task.NewService(task.NewRepository(db))),
		dashboard.NewHandler(db),
		tickets.NewHandler(tickets.NewService(tickets.NewRepository(db))),
	)
	serverErrors := make(chan error, 1)
	app := fx.New(fx.Invoke(func(lifecycle fx.Lifecycle) {
		lifecycle.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					serverErrors <- server.Start(ctx)
				}()
				return nil
			},
			OnStop: func(shutdownCtx context.Context) error {
				return errors.Join(server.Shutdown(shutdownCtx), database.Close(db))
			},
		})
	}))
	if err := app.Start(ctx); err != nil {
		logger.Error("application start failed", "error", err.Error())
		return 1
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	serverErr := waitForShutdown(signals, serverErrors)
	if serverErr != nil {
		logger.Error("server stopped unexpectedly", "error", serverErr.Error())
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer stop()
	if err := app.Stop(shutdownCtx); err != nil {
		logger.Error("application shutdown failed", "error", err.Error())
		return 1
	}
	if serverErr != nil {
		return 1
	}
	return 0
}

func waitForShutdown(signals <-chan os.Signal, serverErrors <-chan error) error {
	if err, ready := receiveServerError(serverErrors); ready {
		return err
	}
	select {
	case <-signals:
		if err, ready := receiveServerError(serverErrors); ready {
			return err
		}
		return nil
	case err, ok := <-serverErrors:
		return serverFailure(err, ok)
	}
}

func receiveServerError(serverErrors <-chan error) (error, bool) {
	select {
	case err, ok := <-serverErrors:
		return serverFailure(err, ok), true
	default:
		return nil, false
	}
}

func serverFailure(err error, ok bool) error {
	if !ok || err == nil {
		return errors.New("HTTP server stopped unexpectedly")
	}
	return fmt.Errorf("HTTP server failed: %w", err)
}
