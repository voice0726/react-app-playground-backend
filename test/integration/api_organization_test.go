//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/dashboard"
	"github.com/voice0726/react-app-playground-backend/internal/deal"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
	"github.com/voice0726/react-app-playground-backend/internal/organization"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/task"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newIntegrationServer(t *testing.T) (*httpserver.Server, *gorm.DB) {
	t.Helper()
	db := openDB(t)
	ensureUp(t, db)
	resetBusinessTables(t, db)
	url := os.Getenv("DATABASE_URL")
	gormDB, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimPrefix(r.Header.Get("Cookie"), "session=")
		if userID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"user_id":"`+userID+`","name":"`+userID+`","email":"`+userID+`@example.com"}`)
	}))
	t.Cleanup(gateway.Close)
	cfg := config.Config{AuthGatewayURL: gateway.URL, AuthGatewayInternalToken: "integration-token", AuthGatewayTimeout: time.Second, PublicOrigin: "http://localhost:5173"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	organizationService := organization.NewService(organization.NewRepository(gormDB))
	server := httpserver.New(cfg, gormDB, logger, authgateway.NewClient(cfg), organizationService)
	customerService := customer.NewService(customer.NewRepository(gormDB))
	server.Register(
		organization.NewHandler(organizationService),
		customer.NewHandler(customerService),
		deal.NewHandler(deal.NewService(deal.NewRepository(gormDB), customerService)),
		invoice.NewHandler(invoice.NewService(invoice.NewRepository(gormDB), customerService)),
		task.NewHandler(task.NewService(task.NewRepository(gormDB))),
		dashboard.NewHandler(gormDB),
	)
	return server, gormDB
}

func integrationRequest(t *testing.T, server *httpserver.Server, method, path, user string, organizationID *uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Cookie", "session="+user)
	if organizationID != nil {
		request.Header.Set("X-Organization-ID", organizationID.String())
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete {
		request.Header.Set("Origin", "http://localhost:5173")
	}
	response := httptest.NewRecorder()
	server.Echo.ServeHTTP(response, request)
	return response
}

func mustOrganization(t *testing.T, response *httptest.ResponseRecorder) (organization.OrganizationResponse, uuid.UUID) {
	t.Helper()
	var value organization.OrganizationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("organization response: %v body=%s", err, response.Body.String())
	}
	organizationID, err := uuid.Parse(value.ID)
	if err != nil {
		t.Fatalf("organization id: %v", err)
	}
	return value, organizationID
}

func TestOrganizationRBACAndSharedResources(t *testing.T) {
	server, _ := newIntegrationServer(t)
	created := integrationRequest(t, server, http.MethodPost, "/api/organizations", "user-a", nil, `{"name":"Operations"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create organization status=%d body=%s", created.Code, created.Body.String())
	}
	organizationValue, organizationID := mustOrganization(t, created)
	if organizationValue.Role != organization.RoleAdmin {
		t.Fatalf("created organization role=%q", organizationValue.Role)
	}
	customerResponse := integrationRequest(t, server, http.MethodPost, "/api/customers", "user-a", &organizationID, `{"name":"Shared customer","email":"shared@example.com","phone":"1","status":"active"}`)
	if customerResponse.Code != http.StatusCreated {
		t.Fatalf("create customer status=%d body=%s", customerResponse.Code, customerResponse.Body.String())
	}
	var customerValue customer.Response
	if err := json.Unmarshal(customerResponse.Body.Bytes(), &customerValue); err != nil {
		t.Fatal(err)
	}

	if response := integrationRequest(t, server, http.MethodGet, "/api/customers", "user-b", &organizationID, ""); response.Code != http.StatusNotFound {
		t.Fatalf("outsider list status=%d body=%s", response.Code, response.Body.String())
	}
	memberPath := "/api/organizations/" + organizationID.String() + "/members"
	memberResponse := integrationRequest(t, server, http.MethodPost, memberPath, "user-a", &organizationID, `{"userId":"user-b","role":"viewer"}`)
	if memberResponse.Code != http.StatusCreated {
		t.Fatalf("add viewer status=%d body=%s", memberResponse.Code, memberResponse.Body.String())
	}
	shared := integrationRequest(t, server, http.MethodGet, "/api/customers", "user-b", &organizationID, "")
	if shared.Code != http.StatusOK || !strings.Contains(shared.Body.String(), "Shared customer") {
		t.Fatalf("shared list status=%d body=%s", shared.Code, shared.Body.String())
	}
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		path := "/api/customers"
		body := `{"name":"viewer","email":"viewer@example.com","phone":"2","status":"active"}`
		if method != http.MethodPost {
			path += "/" + customerValue.ID
			body = `{"name":"changed"}`
		}
		response := integrationRequest(t, server, method, path, "user-b", &organizationID, body)
		if response.Code != http.StatusForbidden {
			t.Fatalf("viewer %s status=%d body=%s", method, response.Code, response.Body.String())
		}
	}
	if response := integrationRequest(t, server, http.MethodPatch, memberPath+"/user-b", "user-a", &organizationID, `{"role":"editor"}`); response.Code != http.StatusOK {
		t.Fatalf("promote editor status=%d body=%s", response.Code, response.Body.String())
	}
	createdByEditor := integrationRequest(t, server, http.MethodPost, "/api/customers", "user-b", &organizationID, `{"name":"Editor customer","email":"editor@example.com","phone":"3","status":"active"}`)
	if createdByEditor.Code != http.StatusCreated {
		t.Fatalf("editor create status=%d body=%s", createdByEditor.Code, createdByEditor.Body.String())
	}
	if response := integrationRequest(t, server, http.MethodPost, memberPath, "user-b", &organizationID, `{"userId":"user-c","role":"viewer"}`); response.Code != http.StatusForbidden {
		t.Fatalf("editor member write status=%d body=%s", response.Code, response.Body.String())
	}
	privateResponse := integrationRequest(t, server, http.MethodPost, "/api/organizations", "user-b", nil, `{"name":"Private"}`)
	if privateResponse.Code != http.StatusCreated {
		t.Fatalf("private organization status=%d body=%s", privateResponse.Code, privateResponse.Body.String())
	}
	_, privateID := mustOrganization(t, privateResponse)
	if response := integrationRequest(t, server, http.MethodGet, "/api/customers", "user-a", &privateID, ""); response.Code != http.StatusNotFound {
		t.Fatalf("cross-organization list status=%d", response.Code)
	}
	if response := integrationRequest(t, server, http.MethodGet, memberPath, "user-c", &organizationID, ""); response.Code != http.StatusNotFound {
		t.Fatalf("outsider members status=%d", response.Code)
	}
	if response := integrationRequest(t, server, http.MethodPost, memberPath, "user-a", &organizationID, `{"userId":"user-c","role":"admin"}`); response.Code != http.StatusCreated {
		t.Fatalf("add second admin status=%d body=%s", response.Code, response.Body.String())
	}
	if response := integrationRequest(t, server, http.MethodDelete, memberPath+"/user-c", "user-a", &organizationID, ""); response.Code != http.StatusOK {
		t.Fatalf("remove second admin status=%d body=%s", response.Code, response.Body.String())
	}
	if response := integrationRequest(t, server, http.MethodPatch, memberPath+"/user-a", "user-a", &organizationID, `{"role":"viewer"}`); response.Code != http.StatusConflict {
		t.Fatalf("demote last admin status=%d body=%s", response.Code, response.Body.String())
	}
	if response := integrationRequest(t, server, http.MethodDelete, memberPath+"/user-a", "user-a", &organizationID, ""); response.Code != http.StatusConflict {
		t.Fatalf("remove last admin status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOrganizationPagingTotalsAndSummary(t *testing.T) {
	server, _ := newIntegrationServer(t)
	created := integrationRequest(t, server, http.MethodPost, "/api/organizations", "user-a", nil, `{"name":"Paging"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create organization status=%d body=%s", created.Code, created.Body.String())
	}
	_, organizationID := mustOrganization(t, created)
	var customerID string
	for index := 1; index <= 12; index++ {
		response := integrationRequest(t, server, http.MethodPost, "/api/customers", "user-a", &organizationID, `{"name":"Paging-`+formatIndex(index)+`","email":"paging`+formatIndex(index)+`@example.com","phone":"1","status":"active"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create customer %d status=%d body=%s", index, response.Code, response.Body.String())
		}
		if index == 1 {
			var value customer.Response
			if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			customerID = value.ID
		}
	}
	pageResponse := integrationRequest(t, server, http.MethodGet, "/api/customers?page=2&pageSize=10&q=Paging&sort=name&order=asc", "user-a", &organizationID, "")
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), `"total":12`) || !strings.Contains(pageResponse.Body.String(), `"page":2`) {
		t.Fatalf("customer page status=%d body=%s", pageResponse.Code, pageResponse.Body.String())
	}
	for _, value := range []struct {
		number string
		amount string
		due    string
		status string
	}{
		{number: "DRAFT-1", amount: "100", due: "2026-06-10", status: "draft"},
		{number: "SENT-OVERDUE", amount: "200", due: "2026-01-01", status: "sent"},
		{number: "SENT-CURRENT", amount: "300", due: "2026-12-01", status: "sent"},
	} {
		response := integrationRequest(t, server, http.MethodPost, "/api/invoices", "user-a", &organizationID, `{"number":"`+value.number+`","customerId":"`+customerID+`","amount":`+value.amount+`,"issueDate":"2026-01-01","dueDate":"`+value.due+`","status":"`+value.status+`"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create invoice %s status=%d body=%s", value.number, response.Code, response.Body.String())
		}
	}
	invoicePage := integrationRequest(t, server, http.MethodGet, "/api/invoices?page=1&pageSize=2&status=sent&today=2026-06-01", "user-a", &organizationID, "")
	if invoicePage.Code != http.StatusOK || !strings.Contains(invoicePage.Body.String(), `"total":2`) || !strings.Contains(invoicePage.Body.String(), `"totalAmount":500`) {
		t.Fatalf("invoice page status=%d body=%s", invoicePage.Code, invoicePage.Body.String())
	}
	overdue := integrationRequest(t, server, http.MethodGet, "/api/invoices?overdue=true&today=2026-06-01", "user-a", &organizationID, "")
	if overdue.Code != http.StatusOK || strings.Count(overdue.Body.String(), "SENT-OVERDUE") != 1 {
		t.Fatalf("overdue list status=%d body=%s", overdue.Code, overdue.Body.String())
	}
	summaryResponse := integrationRequest(t, server, http.MethodGet, "/api/invoices/summary?today=2026-06-01", "user-a", &organizationID, "")
	var summary invoice.Summary
	if summaryResponse.Code != http.StatusOK || json.Unmarshal(summaryResponse.Body.Bytes(), &summary) != nil || summary.DraftCount != 1 || summary.OutstandingAmount != 500 || summary.OverdueCount != 1 || summary.OverdueAmount != 200 {
		t.Fatalf("summary status=%d body=%s summary=%+v", summaryResponse.Code, summaryResponse.Body.String(), summary)
	}
	dashboardResponse := integrationRequest(t, server, http.MethodGet, "/api/dashboard?today=2026-06-01", "user-a", &organizationID, "")
	if dashboardResponse.Code != http.StatusOK || !strings.Contains(dashboardResponse.Body.String(), `"customers":12`) || !strings.Contains(dashboardResponse.Body.String(), `"invoices":3`) || !strings.Contains(dashboardResponse.Body.String(), `"overdueAmount":200`) {
		t.Fatalf("dashboard status=%d body=%s", dashboardResponse.Code, dashboardResponse.Body.String())
	}
}

func TestDealCustomerSortAcrossPages(t *testing.T) {
	server, _ := newIntegrationServer(t)
	created := integrationRequest(t, server, http.MethodPost, "/api/organizations", "user-a", nil, `{"name":"Customer sort"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create organization status=%d body=%s", created.Code, created.Body.String())
	}
	_, organizationID := mustOrganization(t, created)
	var customerIDs []string
	for _, name := range []string{"First", "Second"} {
		response := integrationRequest(t, server, http.MethodPost, "/api/customers", "user-a", &organizationID, `{"name":"`+name+`","email":"sort@example.com","phone":"1","status":"active"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create customer status=%d body=%s", response.Code, response.Body.String())
		}
		var value customer.Response
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		customerIDs = append(customerIDs, value.ID)
		response = integrationRequest(t, server, http.MethodPost, "/api/deals", "user-a", &organizationID, `{"title":"Same title","customerId":"`+value.ID+`","amount":100,"status":"open"}`)
		if response.Code != http.StatusCreated {
			t.Fatalf("create deal status=%d body=%s", response.Code, response.Body.String())
		}
	}
	sort.Strings(customerIDs)
	for _, order := range []string{"asc", "desc"} {
		for index := 0; index < len(customerIDs); index++ {
			response := integrationRequest(t, server, http.MethodGet, "/api/deals?page="+formatIndex(index+1)+"&pageSize=1&sort=customerId&order="+order, "user-a", &organizationID, "")
			var result struct {
				Items []deal.Response `json:"items"`
				Total int             `json:"total"`
			}
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Total != 2 || len(result.Items) != 1 {
				t.Fatalf("deal page status=%d body=%s", response.Code, response.Body.String())
			}
			wantIndex := index
			if order == "desc" {
				wantIndex = len(customerIDs) - 1 - index
			}
			if actual := result.Items[0].CustomerID; actual != customerIDs[wantIndex] {
				t.Fatalf("%s page %d customer=%s, want %s", order, index+1, actual, customerIDs[wantIndex])
			}
		}
	}
}

func TestConcurrentLastAdminMutationsKeepOneAdmin(t *testing.T) {
	server, db := newIntegrationServer(t)
	created := integrationRequest(t, server, http.MethodPost, "/api/organizations", "user-a", nil, `{"name":"Concurrent admin"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create organization status=%d body=%s", created.Code, created.Body.String())
	}
	_, organizationID := mustOrganization(t, created)
	memberPath := "/api/organizations/" + organizationID.String() + "/members"
	if response := integrationRequest(t, server, http.MethodPost, memberPath, "user-a", &organizationID, `{"userId":"user-b","role":"admin"}`); response.Code != http.StatusCreated {
		t.Fatalf("add second admin status=%d body=%s", response.Code, response.Body.String())
	}

	repository := organization.NewRepository(db)
	start := make(chan struct{})
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		<-start
		_, err := repository.UpdateMemberRole(context.Background(), "user-a", organizationID, "user-a", organization.RoleViewer)
		results <- err
	}()
	go func() {
		defer waitGroup.Done()
		<-start
		results <- repository.RemoveMember(context.Background(), "user-a", organizationID, "user-b")
	}()
	close(start)
	waitGroup.Wait()
	close(results)

	succeeded := 0
	failed := 0
	for err := range results {
		if err == nil {
			succeeded++
		} else {
			failed++
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("concurrent mutations succeeded=%d failed=%d, want one of each", succeeded, failed)
	}
	var admins int64
	if err := db.Table("organization_members").Where("organization_id = ? AND role = ?", organizationID, organization.RoleAdmin).Count(&admins).Error; err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Fatalf("concurrent mutations left %d administrators, want 1", admins)
	}
}

func formatIndex(index int) string {
	if index < 10 {
		return "0" + string(rune('0'+index))
	}
	return string(rune('0'+index/10)) + string(rune('0'+index%10))
}
