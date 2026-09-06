package component

import (
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
	s := httpserver.New(cfg, nil, nilLogger(), authgateway.NewClient(cfg), componentOrganizationAuthorizer{})
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
