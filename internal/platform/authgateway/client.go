package authgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/requestuser"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{cfg.AuthGatewayURL, cfg.AuthGatewayInternalToken, &http.Client{Timeout: cfg.AuthGatewayTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) GetSession(ctx context.Context, cookie string) (requestuser.AuthenticatedUser, error) {
	if cookie == "" {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/session", nil)
	if err != nil {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayUnavailable, "auth_gateway_unavailable", "Authentication service unavailable.", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-Internal-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayUnavailable, "auth_gateway_timeout", "Authentication service unavailable.", err)
		}
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayUnavailable, "auth_gateway_unavailable", "Authentication service unavailable.", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.Unauthenticated, "unauthenticated", "Authentication required.", nil)
	}
	if resp.StatusCode >= 500 {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayUnavailable, "auth_gateway_unavailable", "Authentication service unavailable.", fmt.Errorf("status %d", resp.StatusCode))
	}
	if resp.StatusCode != http.StatusOK {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayContract, "auth_gateway_contract", "Authentication service returned an invalid response.", nil)
	}
	var payload struct {
		UserID string   `json:"user_id"`
		Name   string   `json:"name"`
		Email  string   `json:"email"`
		Roles  []string `json:"roles"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil || payload.UserID == "" || payload.Name == "" || payload.Email == "" {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayContract, "auth_gateway_contract", "Authentication service returned an invalid response.", nil)
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return requestuser.AuthenticatedUser{}, httperror.New(httperror.AuthGatewayContract, "auth_gateway_contract", "Authentication service returned an invalid response.", nil)
	}
	return requestuser.AuthenticatedUser{ID: payload.UserID, Name: payload.Name, Email: payload.Email, Roles: payload.Roles}, nil
}

type Middleware struct{ client *Client }

func NewMiddleware(client *Client) *Middleware { return &Middleware{client} }
func (m *Middleware) Handler(next func(context.Context, requestuser.AuthenticatedUser) error) func(context.Context, string) error {
	return func(ctx context.Context, cookie string) error {
		user, err := m.client.GetSession(ctx, cookie)
		if err != nil {
			return err
		}
		return next(ctx, user)
	}
}

var _ = time.Second

func (c *Client) EchoMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx *echo.Context) error {
		user, err := c.GetSession(ctx.Request().Context(), ctx.Request().Header.Get("Cookie"))
		if err != nil {
			return err
		}
		ctx.Set("authenticated_user", user)
		ctx.SetRequest(ctx.Request().WithContext(requestuser.With(ctx.Request().Context(), user)))
		return next(ctx)
	}
}
