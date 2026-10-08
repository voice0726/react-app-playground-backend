package component

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"github.com/voice0726/react-app-playground-backend/internal/tickets/tickettest"
)

type ticketAPI struct {
	t      *testing.T
	server *httpserver.Server
	origin string
}

func newTicketAPI(t *testing.T) *ticketAPI {
	t.Helper()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := "user-a"
		if r.Header.Get("Cookie") == "session=b" {
			user = "user-b"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"` + user + `","name":"Name ` + user + `","email":"` + user + `@example.com"}`))
	}))
	t.Cleanup(gateway.Close)
	cfg := config.Config{AuthGatewayURL: gateway.URL, AuthGatewayInternalToken: "component-token", AuthGatewayTimeout: time.Second, PublicOrigin: "http://localhost:5173"}
	s := httpserver.New(cfg, nil, nilLogger(), authgateway.NewClient(cfg))
	s.Register(tickets.NewHandler(tickets.NewService(tickettest.NewRepository())))
	return &ticketAPI{t: t, server: s, origin: cfg.PublicOrigin}
}

func (a *ticketAPI) do(cookie, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Origin", a.origin)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	a.server.Echo.ServeHTTP(res, req)
	return res
}

func (a *ticketAPI) json(res *httptest.ResponseRecorder, want int) map[string]any {
	a.t.Helper()
	if res.Code != want {
		a.t.Fatalf("status=%d, want %d, body=%s", res.Code, want, res.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		a.t.Fatalf("body is not a JSON object: %s", res.Body.String())
	}
	return out
}

func errorCode(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error body: %s", res.Body.String())
	}
	return body.Error.Code
}

func TestTicketsContractAndLifecycle(t *testing.T) {
	a := newTicketAPI(t)

	created := a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"  Bug  ","assignee":"bob"}`), http.StatusCreated)
	if created["id"] != float64(1) || created["title"] != "Bug" || created["status"] != "open" || created["assignee"] != "bob" {
		t.Fatalf("created=%v", created)
	}
	actor := map[string]any{"id": "user-a", "email": "user-a@example.com", "displayName": "Name user-a"}
	for _, field := range []string{"createdBy", "updatedBy"} {
		if got := created[field].(map[string]any); got["id"] != actor["id"] || got["email"] != actor["email"] || got["displayName"] != actor["displayName"] {
			t.Fatalf("%s=%v", field, got)
		}
	}
	if _, err := time.Parse(time.RFC3339, created["createdAt"].(string)); err != nil {
		t.Fatalf("createdAt: %v", err)
	}
	if h := created["history"].(map[string]any)["items"]; len(h.([]any)) != 0 {
		t.Fatalf("history=%v", h)
	}
	if c := created["comments"].(map[string]any)["items"]; len(c.([]any)) != 0 {
		t.Fatalf("comments=%v", c)
	}

	patched := a.json(a.do("session=a", http.MethodPatch, "/api/tickets/1", `{"status":"closed","assignee":null}`), http.StatusOK)
	if _, ok := patched["assignee"]; ok || patched["status"] != "closed" {
		t.Fatalf("patched=%v", patched)
	}
	change := patched["history"].(map[string]any)["items"].([]any)[0].(map[string]any)
	if change["operationId"] == "" || change["actor"].(map[string]any)["id"] != "user-a" {
		t.Fatalf("history item=%v", change)
	}
	first := change["changes"].([]any)[0].(map[string]any)
	if first["field"] != "status" || first["before"] != "open" || first["after"] != "closed" {
		t.Fatalf("change=%v", first)
	}

	replaced := a.json(a.do("session=a", http.MethodPut, "/api/tickets/1", `{"title":"Bug 2","status":"open","assignee":"carol"}`), http.StatusOK)
	if replaced["title"] != "Bug 2" || replaced["assignee"] != "carol" || len(replaced["history"].(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("replaced=%v", replaced)
	}

	withComment := a.json(a.do("session=a", http.MethodPost, "/api/tickets/1/comments", `{"body":"hello"}`), http.StatusCreated)
	comment := withComment["comments"].(map[string]any)["items"].([]any)[0].(map[string]any)
	if comment["body"] != "hello" || comment["createdBy"].(map[string]any)["id"] != "user-a" {
		t.Fatalf("comment=%v", comment)
	}
	commentPath := "/api/tickets/1/comments/1"
	edited := a.json(a.do("session=a", http.MethodPut, commentPath, `{"body":"edited"}`), http.StatusOK)
	if edited["comments"].(map[string]any)["items"].([]any)[0].(map[string]any)["body"] != "edited" {
		t.Fatalf("edited=%v", edited)
	}
	removed := a.json(a.do("session=a", http.MethodDelete, commentPath, ""), http.StatusOK)
	if len(removed["comments"].(map[string]any)["items"].([]any)) != 0 {
		t.Fatalf("removed=%v", removed)
	}

	list := a.json(a.do("session=a", http.MethodGet, "/api/tickets?q=bug&sortBy=updated_at&sortOrder=dsc", ""), http.StatusOK)
	items := list["items"].([]any)
	if list["total"] != float64(1) || len(items) != 1 {
		t.Fatalf("list=%v", list)
	}
	if _, ok := items[0].(map[string]any)["history"]; ok {
		t.Fatalf("list items must be summaries: %v", items[0])
	}

	if res := a.do("session=a", http.MethodDelete, "/api/tickets/1", ""); res.Code != http.StatusNoContent || res.Body.Len() != 0 {
		t.Fatalf("delete: %d %q", res.Code, res.Body.String())
	}
	if res := a.do("session=a", http.MethodGet, "/api/tickets/1", ""); res.Code != http.StatusNotFound || errorCode(t, res) != "ticket_not_found" {
		t.Fatalf("after delete: %d %s", res.Code, res.Body.String())
	}
}

func TestTicketsAreIsolatedPerUser(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"private"}`), http.StatusCreated)

	list := a.json(a.do("session=b", http.MethodGet, "/api/tickets", ""), http.StatusOK)
	if list["total"] != float64(0) {
		t.Fatalf("list=%v", list)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/tickets/1", ""},
		{http.MethodPut, "/api/tickets/1", `{"title":"x","status":"open"}`},
		{http.MethodPatch, "/api/tickets/1", `{"title":"x"}`},
		{http.MethodDelete, "/api/tickets/1", ""},
		{http.MethodPost, "/api/tickets/1/comments", `{"body":"x"}`},
	} {
		if res := a.do("session=b", tc.method, tc.path, tc.body); res.Code != http.StatusNotFound {
			t.Errorf("%s %s: status=%d", tc.method, tc.path, res.Code)
		}
	}
	a.json(a.do("session=a", http.MethodGet, "/api/tickets/1", ""), http.StatusOK)
}

func TestTicketsErrorClassification(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"t"}`), http.StatusCreated)
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"blank title", http.MethodPost, "/api/tickets", `{"title":" "}`, 400, "invalid_title"},
		{"bad status", http.MethodPost, "/api/tickets", `{"title":"x","status":"pending"}`, 400, "invalid_status"},
		{"unknown field", http.MethodPost, "/api/tickets", `{"title":"x","owner":"y"}`, 400, "invalid_json"},
		{"wrong type", http.MethodPatch, "/api/tickets/1", `{"title":1}`, 400, "invalid_json"},
		{"empty patch", http.MethodPatch, "/api/tickets/1", `{}`, 400, "empty_patch"},
		{"null title", http.MethodPatch, "/api/tickets/1", `{"title":null}`, 400, "invalid_title"},
		{"put without status", http.MethodPut, "/api/tickets/1", `{"title":"x"}`, 400, "invalid_status"},
		{"blank comment", http.MethodPost, "/api/tickets/1/comments", `{"body":" "}`, 400, "invalid_comment_body"},
		{"missing ticket", http.MethodGet, "/api/tickets/42", "", 404, "ticket_not_found"},
		{"non numeric ticket", http.MethodGet, "/api/tickets/abc", "", 404, "ticket_not_found"},
		{"missing comment", http.MethodPut, "/api/tickets/1/comments/9", `{"body":"x"}`, 404, "comment_not_found"},
		{"no cookie", http.MethodGet, "/api/tickets", "", 401, "unauthenticated"},
	} {
		cookie := "session=a"
		if tc.name == "no cookie" {
			cookie = ""
		}
		res := a.do(cookie, tc.method, tc.path, tc.body)
		if res.Code != tc.status || errorCode(t, res) != tc.code {
			t.Errorf("%s: status=%d body=%s, want %d %s", tc.name, res.Code, res.Body.String(), tc.status, tc.code)
		}
	}
}

func TestTicketsOriginGuardCoversPut(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"t"}`), http.StatusCreated)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets/1/comments", `{"body":"c"}`), http.StatusCreated)
	a.origin = "http://evil.example"
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/tickets/1", `{"title":"x","status":"open"}`},
		{http.MethodPut, "/api/tickets/1/comments/1", `{"body":"x"}`},
		{http.MethodPatch, "/api/tickets/1", `{"title":"x"}`},
		{http.MethodDelete, "/api/tickets/1", ""},
		{http.MethodPost, "/api/tickets", `{"title":"x"}`},
	} {
		if res := a.do("session=a", tc.method, tc.path, tc.body); res.Code != http.StatusForbidden || errorCode(t, res) != "origin_forbidden" {
			t.Errorf("%s %s: status=%d body=%s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}
}
