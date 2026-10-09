package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/deal"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/database"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/platform/logging"
	"github.com/voice0726/react-app-playground-backend/internal/task"
	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"go.uber.org/fx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	logger := logging.New(cfg)
	connectionTimeout, err := database.ConnectionTimeout()
	if err != nil {
		logger.Error("invalid database connection timeout", "error", err.Error())
		os.Exit(1)
	}
	db, err := database.New(cfg, logger)
	if err != nil {
		logger.Error("database connection failed", "error", err.Error())
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("database initialization failed", "error", err.Error())
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connectCtx, stopConnecting := context.WithTimeout(ctx, connectionTimeout)
	err = database.WaitForConnection(connectCtx, sqlDB, logger)
	stopConnecting()
	if err != nil {
		_ = sqlDB.Close()
		logger.Error("database unavailable", "error", err.Error())
		os.Exit(1)
	}
	server := httpserver.New(cfg, db, logger, authgateway.NewClient(cfg))
	customerService := customer.NewService(customer.NewRepository(db))
	server.Register(
		customer.NewHandler(customerService),
		deal.NewHandler(deal.NewService(deal.NewRepository(db), customerService)),
		invoice.NewHandler(invoice.NewService(invoice.NewRepository(db), customerService)),
		task.NewHandler(task.NewService(task.NewRepository(db))),
		tickets.NewHandler(tickets.NewService(tickets.NewRepository(db))),
	)
	app := fx.New(fx.Invoke(func(lifecycle fx.Lifecycle) {
		lifecycle.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go func() {
					if err := server.Start(ctx); err != nil {
						logger.Error("server stopped", "error", err.Error())
					}
				}()
				return nil
			},
			OnStop: func(shutdownCtx context.Context) error {
				if err := server.Shutdown(shutdownCtx); err != nil {
					return err
				}
				return database.Close(db)
			},
		})
	}))
	if err := app.Start(ctx); err != nil {
		logger.Error("application start failed", "error", err.Error())
		os.Exit(1)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	select {
	case <-signals:
	case <-ctx.Done():
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer stop()
	_ = app.Stop(shutdownCtx)
}
