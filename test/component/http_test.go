package component

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httpjson"
)

type registrar func(*echo.Group)

func (r registrar) Register(g *echo.Group) { r(g) }

func newServer(t *testing.T, gatewayStatus int, gatewayBody string) *httpserver.Server {
	t.Helper()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/session" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(gatewayStatus)
		_, _ = w.Write([]byte(gatewayBody))
	}))
	t.Cleanup(gateway.Close)
	cfg := config.Config{AuthGatewayURL: gateway.URL, AuthGatewayInternalToken: "component-token", AuthGatewayTimeout: time.Second, PublicOrigin: "http://localhost:5173"}
	s := httpserver.New(cfg, nil, nilLogger(), authgateway.NewClient(cfg))
	return s
}

func nilLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHealthAndReadiness(t *testing.T) {
	s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
	for _, tc := range []struct {
		path string
		want int
	}{{"/healthz", http.StatusOK}, {"/readyz", http.StatusServiceUnavailable}} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		res := httptest.NewRecorder()
		s.Echo.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Fatalf("%s: status=%d, want=%d", tc.path, res.Code, tc.want)
		}
	}
}

func TestHTTPErrorHandlerContracts(t *testing.T) {
	s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
	s.Echo.GET("/echo-error", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query.")
	})
	s.Echo.GET("/wrapped-echo-error", func(c *echo.Context) error {
		return fmt.Errorf("route failed: %w", echo.NewHTTPError(http.StatusNotFound, "Record not found."))
	})
	s.Echo.GET("/echo-internal-error", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "private echo detail")
	})
	s.Echo.GET("/echo-bad-gateway", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusBadGateway, "private upstream detail")
	})
	s.Echo.GET("/invalid-echo-status", func(c *echo.Context) error {
		return echo.NewHTTPError(http.StatusMovedPermanently, "private invalid status detail")
	})
	s.Echo.GET("/domain-error", func(c *echo.Context) error {
		return httperror.New(httperror.Validation, "invalid_input", "Input is invalid.", nil)
	})
	s.Echo.GET("/internal-error", func(c *echo.Context) error {
		return httperror.New(httperror.Internal, "internal_error", "Internal server error.", fmt.Errorf("private database detail"))
	})
	s.Echo.GET("/committed-error", func(c *echo.Context) error {
		if err := c.String(http.StatusAccepted, "response already sent"); err != nil {
			return err
		}
		return fmt.Errorf("error after response was sent")
	})
	s.Echo.GET("/plain-error", func(c *echo.Context) error {
		return fmt.Errorf("private plain error detail")
	})

	for _, tc := range []struct {
		name, method, path, wantCode, wantMessage, forbidden string
		wantStatus                                           int
	}{
		{name: "missing route", method: http.MethodGet, path: "/missing", wantStatus: http.StatusNotFound, wantCode: "not_found", wantMessage: "Not Found"},
		{name: "unsupported method", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed, wantCode: "method_not_allowed", wantMessage: "Method Not Allowed"},
		{name: "echo http error", method: http.MethodGet, path: "/echo-error", wantStatus: http.StatusBadRequest, wantCode: "bad_request", wantMessage: "Invalid query."},
		{name: "wrapped echo error", method: http.MethodGet, path: "/wrapped-echo-error", wantStatus: http.StatusNotFound, wantCode: "not_found", wantMessage: "Record not found."},
		{name: "echo internal error", method: http.MethodGet, path: "/echo-internal-error", wantStatus: http.StatusInternalServerError, wantCode: "internal_error", wantMessage: "Internal server error.", forbidden: "private echo detail"},
		{name: "echo bad gateway", method: http.MethodGet, path: "/echo-bad-gateway", wantStatus: http.StatusBadGateway, wantCode: "bad_gateway", wantMessage: "Bad Gateway", forbidden: "private upstream detail"},
		{name: "invalid echo status", method: http.MethodGet, path: "/invalid-echo-status", wantStatus: http.StatusInternalServerError, wantCode: "internal_error", wantMessage: "Internal server error.", forbidden: "private invalid status detail"},
		{name: "domain error", method: http.MethodGet, path: "/domain-error", wantStatus: http.StatusBadRequest, wantCode: "invalid_input", wantMessage: "Input is invalid."},
		{name: "internal error", method: http.MethodGet, path: "/internal-error", wantStatus: http.StatusInternalServerError, wantCode: "internal_error", wantMessage: "Internal server error.", forbidden: "private database detail"},
		{name: "plain error", method: http.MethodGet, path: "/plain-error", wantStatus: http.StatusInternalServerError, wantCode: "internal_error", wantMessage: "Internal server error.", forbidden: "private plain error detail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			s.Echo.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != tc.wantStatus {
				t.Fatalf("status=%d, want=%d body=%s", res.Code, tc.wantStatus, res.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Error.Code != tc.wantCode || body.Error.Message != tc.wantMessage {
				t.Fatalf("error=%+v, want code=%q message=%q", body.Error, tc.wantCode, tc.wantMessage)
			}
			if tc.forbidden != "" && strings.Contains(res.Body.String(), tc.forbidden) {
				t.Fatalf("response leaked internal detail %q: %s", tc.forbidden, res.Body.String())
			}
		})
	}

	res := httptest.NewRecorder()
	s.Echo.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/committed-error", nil))
	if res.Code != http.StatusAccepted || res.Body.String() != "response already sent" {
		t.Fatalf("committed response changed: status=%d body=%q", res.Code, res.Body.String())
	}
}

func TestAuthClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{"no cookie", http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`, http.StatusUnauthorized},
		{"gateway unauthorized", http.StatusUnauthorized, `{"error":"unauthorized"}`, http.StatusUnauthorized},
		{"gateway contract", http.StatusOK, `{"user_id":""}`, http.StatusBadGateway},
		{"gateway unavailable", http.StatusBadGateway, `{"error":"down"}`, http.StatusServiceUnavailable},
		{"gateway service unavailable", http.StatusServiceUnavailable, `{"error":"down"}`, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer(t, tc.status, tc.body)
			s.Register(registrar(func(g *echo.Group) {
				g.GET("/probe", func(c *echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/probe", nil)
			if tc.name != "no cookie" {
				req.Header.Set("Cookie", "session=component")
			}
			res := httptest.NewRecorder()
			s.Echo.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status=%d, want=%d body=%s", res.Code, tc.want, res.Body.String())
			}
		})
	}
}

func TestOriginGuardAndStrictJSON(t *testing.T) {
	s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
	s.Register(registrar(func(g *echo.Group) {
		g.POST("/strict", func(c *echo.Context) error {
			var body struct {
				Name string `json:"name"`
			}
			if err := httpjson.Decode(c.Request(), &body); err != nil {
				return httperror.New(httperror.Validation, "invalid_json", "Invalid JSON request.", nil)
			}
			return c.JSON(http.StatusOK, body)
		})
	}))
	for _, tc := range []struct {
		name, origin, body string
		want               int
	}{
		{"missing origin", "", `{"name":"x"}`, http.StatusForbidden},
		{"different origin", "https://evil.example", `{"name":"x"}`, http.StatusForbidden},
		{"unknown field", "http://localhost:5173", `{"name":"x","owner_user_id":"other"}`, http.StatusBadRequest},
		{"trailing JSON", "http://localhost:5173", `{"name":"x"}{}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/strict", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			req.Header.Set("Cookie", "session=component")
			res := httptest.NewRecorder()
			s.Echo.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status=%d, want=%d body=%s", res.Code, tc.want, res.Body.String())
			}
		})
	}
}
