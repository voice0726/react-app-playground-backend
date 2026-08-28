package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
	"gorm.io/gorm"
)

type Server struct {
	Echo *echo.Echo
	cfg  config.Config
	db   *gorm.DB
	log  *slog.Logger
	auth *authgateway.Client
	http *http.Server
}
type RouteRegistrar interface{ Register(*echo.Group) }

func New(cfg config.Config, db *gorm.DB, log *slog.Logger, auth *authgateway.Client) *Server {
	e := echo.New()
	e.Logger = log
	e.HTTPErrorHandler = errorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	s := &Server{Echo: e, cfg: cfg, db: db, log: log, auth: auth, http: &http.Server{Addr: cfg.ServerAddr, Handler: e}}
	registerHealth(e, db)
	return s
}
func (s *Server) Register(registrars ...RouteRegistrar) {
	api := s.Echo.Group("/api", s.auth.EchoMiddleware, echo.WrapMiddleware(OriginGuard(s.cfg)))
	for _, registrar := range registrars {
		registrar.Register(api)
	}
}
func (s *Server) Start(ctx context.Context) error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
func registerHealth(e *echo.Echo, db *gorm.DB) {
	e.GET("/healthz", func(c *echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	e.GET("/readyz", func(c *echo.Context) error {
		if db == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request().Context()) != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
}
func errorHandler(c *echo.Context, err error) {
	he := httperrorFrom(err)
	status := map[httperror.Kind]int{httperror.Validation: 400, httperror.Unauthenticated: 401, httperror.NotFound: 404, httperror.Conflict: 409, httperror.AuthGatewayContract: 502, httperror.AuthGatewayUnavailable: 503, httperror.Internal: 500, httperror.Forbidden: 403}[he.Kind]
	_ = c.JSON(status, map[string]interface{}{"error": map[string]string{"code": he.Code, "message": he.Message}})
}
func httperrorFrom(err error) *httperror.Error {
	var e *httperror.Error
	if errors.As(err, &e) {
		return e
	}
	return httperror.New(httperror.Internal, "internal_error", "Internal server error.", err)
}
func writeError(w http.ResponseWriter, err error) error {
	h := httperrorFrom(err)
	status := map[httperror.Kind]int{httperror.Validation: 400, httperror.Unauthenticated: 401, httperror.NotFound: 404, httperror.Conflict: 409, httperror.AuthGatewayContract: 502, httperror.AuthGatewayUnavailable: 503, httperror.Internal: 500, httperror.Forbidden: 403}[h.Kind]
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, writeErr := w.Write([]byte(`{"error":{"code":"` + h.Code + `","message":"` + h.Message + `"}}`))
	return writeErr
}

var _ = requestuser.ErrMissing
var _ = time.Second
