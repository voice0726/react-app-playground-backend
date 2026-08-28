package authgateway

import (
	"context"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetSessionClassifiesGatewayResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		kind   httperror.Kind
	}{{"unauthenticated", 401, "", httperror.Unauthenticated}, {"contract", 200, `{"user_id":""}`, httperror.AuthGatewayContract}, {"unavailable", 503, "", httperror.AuthGatewayUnavailable}, {"success", 200, `{"user_id":"u1","name":"User","email":"u@example.com","roles":[]}`, -1}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cookie") != "session=x" || r.Header.Get("X-Internal-Token") != "token" {
					t.Errorf("headers not forwarded safely")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := NewClient(config.Config{AuthGatewayURL: srv.URL, AuthGatewayInternalToken: "token", AuthGatewayTimeout: time.Second})
			u, e := c.GetSession(context.Background(), "session=x")
			if tc.kind < 0 {
				if e != nil || u.ID != "u1" {
					t.Fatalf("success: %v %#v", e, u)
				}
				return
			}
			if e == nil || httperrorFrom(e).Kind != tc.kind {
				t.Fatalf("error=%v", e)
			}
		})
	}
}
func httperrorFrom(e error) *httperror.Error {
	v, ok := e.(*httperror.Error)
	if !ok {
		return &httperror.Error{Kind: httperror.Internal}
	}
	return v
}
