package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
	"gorm.io/gorm"
)

type Server struct {
	Echo *echo.Echo
	cfg  config.Config
	db   *gorm.DB
	log  *slog.Logger
	auth *authgateway.Client
	org  OrganizationAuthorizer
	http *http.Server
}
type RouteRegistrar interface{ Register(*echo.Group) }
type ScopedRouteRegistrar interface{ RegisterScoped(*echo.Group) }

type OrganizationAuthorizer interface {
	Role(context.Context, string, uuid.UUID) (string, error)
}

func New(cfg config.Config, db *gorm.DB, log *slog.Logger, auth *authgateway.Client, authorizers ...OrganizationAuthorizer) *Server {
	e := echo.New()
	e.Logger = log
	e.HTTPErrorHandler = errorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	var authorizer OrganizationAuthorizer
	if len(authorizers) > 0 {
		authorizer = authorizers[0]
	}
	s := &Server{Echo: e, cfg: cfg, db: db, log: log, auth: auth, org: authorizer, http: &http.Server{Addr: cfg.ServerAddr, Handler: e}}
	registerHealth(e, db)
	return s
}
func (s *Server) Register(registrars ...RouteRegistrar) {
	api := s.Echo.Group("/api", s.auth.EchoMiddleware, echo.WrapMiddleware(OriginGuard(s.cfg)))
	scoped := api.Group("", s.organizationMiddleware)
	for _, registrar := range registrars {
		if scopedRegistrar, ok := registrar.(ScopedRouteRegistrar); ok {
			scopedRegistrar.RegisterScoped(scoped)
			continue
		}
		registrar.Register(api)
	}
}

func (s *Server) organizationMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		user, err := requestuser.From(c.Request().Context())
		if err != nil {
			return httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", err)
		}
		rawID := c.Request().Header.Get("X-Organization-ID")
		if rawID == "" {
			return httperror.New(httperror.Validation, "organization_required", "X-Organization-ID is required.", nil)
		}
		organizationID, err := uuid.Parse(rawID)
		if err != nil {
			return httperror.New(httperror.Validation, "invalid_organization_id", "Invalid organization ID.", nil)
		}
		if s.org == nil {
			return httperror.New(httperror.Internal, "organization_access_unavailable", "Organization access is unavailable.", nil)
		}
		role, err := s.org.Role(c.Request().Context(), user.ID, organizationID)
		if err != nil {
			return err
		}
		if role != "admin" && role != "editor" && role != "viewer" {
			return httperror.New(httperror.Internal, "organization_access_invalid", "Organization access is invalid.", nil)
		}
		if role == "viewer" && c.Request().Method != http.MethodGet && c.Request().Method != http.MethodHead && c.Request().Method != http.MethodOptions {
			return httperror.New(httperror.Forbidden, "organization_write_forbidden", "Viewer permission cannot modify organization data.", nil)
		}
		ctx := orgcontext.With(c.Request().Context(), orgcontext.Scope{OrganizationID: organizationID, UserID: user.ID, Role: role})
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
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
