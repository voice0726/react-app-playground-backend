package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func New(cfg config.Config, log *slog.Logger) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{Logger: newLogger(log), DisableAutomaticPing: true})
}
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type gormLogger struct{ log *slog.Logger }

func newLogger(log *slog.Logger) logger.Interface                  { return &gormLogger{log} }
func (l *gormLogger) LogMode(logger.LogLevel) logger.Interface     { return l }
func (l *gormLogger) Info(context.Context, string, ...interface{}) {}
func (l *gormLogger) Warn(context.Context, string, ...interface{}) {}
func (l *gormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	l.log.ErrorContext(ctx, "database error", "message", msg)
}
func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if err != nil {
		l.log.ErrorContext(ctx, "database query failed", "error", err.Error())
		return
	}
	if time.Since(begin) > time.Second {
		l.log.WarnContext(ctx, "database slow query", "latency", time.Since(begin).String())
	}
}
