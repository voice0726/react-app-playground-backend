package component

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/tickets"
	"github.com/voice0726/react-app-playground-backend/internal/tickets/tickettest"
)

var ticketOtherOrganizationID = uuid.MustParse("22222222-2222-4222-8222-222222222222")

// ticketOrganizationAuthorizer は user-a と user-c を共通の組織に、user-b を別の組織に所属させる。
type ticketOrganizationAuthorizer struct{}

func (ticketOrganizationAuthorizer) Role(_ context.Context, userID string, organizationID uuid.UUID) (string, error) {
	roles := map[string]map[uuid.UUID]string{
		"user-a": {componentOrganizationID: "admin"},
		"user-b": {ticketOtherOrganizationID: "admin"},
		"user-c": {componentOrganizationID: "editor"},
		"user-v": {componentOrganizationID: "viewer"},
	}
	role, ok := roles[userID][organizationID]
	if !ok {
		return "", httperror.New(httperror.NotFound, "organization_not_found", "Organization not found.", nil)
	}
	return role, nil
}

type ticketAPI struct {
	t      *testing.T
	server *httpserver.Server
	origin string
}

func newTicketAPI(t *testing.T) *ticketAPI {
	t.Helper()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := "user-" + strings.TrimPrefix(r.Header.Get("Cookie"), "session=")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"` + user + `","name":"Name ` + user + `","email":"` + user + `@example.com"}`))
	}))
	t.Cleanup(gateway.Close)
	cfg := config.Config{AuthGatewayURL: gateway.URL, AuthGatewayInternalToken: "component-token", AuthGatewayTimeout: time.Second, PublicOrigin: "http://localhost:5173"}
	s := httpserver.New(cfg, nil, nilLogger(), authgateway.NewClient(cfg), ticketOrganizationAuthorizer{})
	s.Register(tickets.NewHandler(tickets.NewService(tickettest.NewRepository())))
	return &ticketAPI{t: t, server: s, origin: cfg.PublicOrigin}
}

// do は session=b を別組織、それ以外を共通組織の利用者として送る。
func (a *ticketAPI) do(cookie, method, path, body string) *httptest.ResponseRecorder {
	organizationID := componentOrganizationID
	if cookie == "session=b" {
		organizationID = ticketOtherOrganizationID
	}
	return a.doIn(organizationID.String(), cookie, method, path, body)
}

func (a *ticketAPI) doIn(organizationID, cookie, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if organizationID != "" {
		req.Header.Set("X-Organization-ID", organizationID)
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

func TestTicketsAreIsolatedPerOrganization(t *testing.T) {
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

	// 所属していない組織を指定しても他組織のチケットには届かない。
	if res := a.doIn(componentOrganizationID.String(), "session=b", http.MethodGet, "/api/tickets/1", ""); res.Code != http.StatusNotFound || errorCode(t, res) != "organization_not_found" {
		t.Fatalf("non-member: %d %s", res.Code, res.Body.String())
	}
}

func TestTicketsAreSharedWithinOrganization(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"shared"}`), http.StatusCreated)

	list := a.json(a.do("session=c", http.MethodGet, "/api/tickets", ""), http.StatusOK)
	if list["total"] != float64(1) {
		t.Fatalf("list=%v", list)
	}
	patched := a.json(a.do("session=c", http.MethodPatch, "/api/tickets/1", `{"status":"closed"}`), http.StatusOK)
	if patched["createdBy"].(map[string]any)["id"] != "user-a" || patched["updatedBy"].(map[string]any)["id"] != "user-c" {
		t.Fatalf("patched=%v", patched)
	}
	a.json(a.do("session=c", http.MethodPost, "/api/tickets/1/comments", `{"body":"by c"}`), http.StatusCreated)
	if res := a.do("session=a", http.MethodPut, "/api/tickets/1/comments/1", `{"body":"x"}`); res.Code != http.StatusForbidden || errorCode(t, res) != "comment_forbidden" {
		t.Fatalf("other member's comment: %d %s", res.Code, res.Body.String())
	}

	a.json(a.do("session=v", http.MethodGet, "/api/tickets/1", ""), http.StatusOK)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/tickets", `{"title":"x"}`},
		{http.MethodPut, "/api/tickets/1", `{"title":"x","status":"open"}`},
		{http.MethodPatch, "/api/tickets/1", `{"title":"x"}`},
		{http.MethodDelete, "/api/tickets/1", ""},
		{http.MethodPost, "/api/tickets/1/comments", `{"body":"x"}`},
	} {
		if res := a.do("session=v", tc.method, tc.path, tc.body); res.Code != http.StatusForbidden || errorCode(t, res) != "organization_write_forbidden" {
			t.Errorf("viewer %s %s: status=%d body=%s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}
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
		{"no organization", http.MethodGet, "/api/tickets", "", 400, "organization_required"},
		{"invalid organization", http.MethodGet, "/api/tickets", "", 400, "invalid_organization_id"},
	} {
		cookie, organizationID := "session=a", componentOrganizationID.String()
		switch tc.name {
		case "no cookie":
			cookie = ""
		case "no organization":
			organizationID = ""
		case "invalid organization":
			organizationID = "not-a-uuid"
		}
		res := a.doIn(organizationID, cookie, tc.method, tc.path, tc.body)
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

func TestTicketsConcurrentIndependentPatchesAreMerged(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"old","assignee":"old"}`), http.StatusCreated)

	bodies := []string{`{"title":"new"}`, `{"assignee":"new"}`}
	responses := make([]*httptest.ResponseRecorder, len(bodies))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, body := range bodies {
		wg.Add(1)
		go func(i int, body string) {
			defer wg.Done()
			<-start
			responses[i] = a.do("session=a", http.MethodPatch, "/api/tickets/1", body)
		}(i, body)
	}
	close(start)
	wg.Wait()
	for _, res := range responses {
		a.json(res, http.StatusOK)
	}

	got := a.json(a.do("session=a", http.MethodGet, "/api/tickets/1", ""), http.StatusOK)
	if got["title"] != "new" || got["assignee"] != "new" {
		t.Fatalf("concurrent patches lost an update: %v", got)
	}
}

func TestTicketsConcurrentCommentAndClosePreserveStatusHistory(t *testing.T) {
	a := newTicketAPI(t)
	a.json(a.do("session=a", http.MethodPost, "/api/tickets", `{"title":"bug"}`), http.StatusCreated)

	requests := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPatch, "/api/tickets/1", `{"status":"closed"}`, http.StatusOK},
		{http.MethodPost, "/api/tickets/1/comments", `{"body":"triaged"}`, http.StatusCreated},
	}
	responses := make([]*httptest.ResponseRecorder, len(requests))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, req := range requests {
		wg.Add(1)
		go func(i int, method, path, body string) {
			defer wg.Done()
			<-start
			responses[i] = a.do("session=a", method, path, body)
		}(i, req.method, req.path, req.body)
	}
	close(start)
	wg.Wait()
	for i, res := range responses {
		a.json(res, requests[i].status)
	}

	got := a.json(a.do("session=a", http.MethodGet, "/api/tickets/1", ""), http.StatusOK)
	if got["status"] != "closed" {
		t.Fatalf("comment overwrote closed status: %v", got)
	}
	history := got["history"].(map[string]any)["items"].([]any)
	if len(history) != 1 {
		t.Fatalf("history=%v, want one status change", history)
	}
	changes := history[0].(map[string]any)["changes"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["before"] != "open" || changes[0].(map[string]any)["after"] != "closed" {
		t.Fatalf("status history=%v", history)
	}
	comments := got["comments"].(map[string]any)["items"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["body"] != "triaged" {
		t.Fatalf("comments=%v", comments)
	}
}
