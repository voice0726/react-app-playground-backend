package component

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/config"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/deal"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
	"github.com/voice0726/react-app-playground-backend/internal/platform/authgateway"
	"github.com/voice0726/react-app-playground-backend/internal/platform/httpserver"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/task"
)

type customerFake struct{ values map[string]customer.Customer }

var componentOrganizationID = uuid.MustParse("11111111-1111-4111-8111-111111111111")

type componentOrganizationAuthorizer struct{}

func (componentOrganizationAuthorizer) Role(_ context.Context, _ string, organizationID uuid.UUID) (string, error) {
	if organizationID != componentOrganizationID {
		return "", httperror.New(httperror.NotFound, "organization_not_found", "Organization not found.", nil)
	}
	return "admin", nil
}

func (f *customerFake) List(_ context.Context, organizationID uuid.UUID, _ customer.ListOptions) ([]customer.Customer, int64, error) {
	out := []customer.Customer{}
	for _, v := range f.values {
		if v.OrganizationID == organizationID {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), nil
}
func (f *customerFake) Find(_ context.Context, organizationID, id uuid.UUID) (customer.Customer, error) {
	v, ok := f.values[organizationID.String()+id.String()]
	if !ok {
		return customer.Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
	}
	return v, nil
}
func (f *customerFake) FindByOrganization(ctx context.Context, organizationID, id uuid.UUID) (customer.Customer, error) {
	return f.Find(ctx, organizationID, id)
}
func (f *customerFake) Create(_ context.Context, organizationID uuid.UUID, user string, v customer.Customer) (customer.Customer, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = user
	v.CreatedAt = time.Now()
	f.values[organizationID.String()+v.ID.String()] = v
	return v, nil
}
func (f *customerFake) Update(ctx context.Context, organizationID, id uuid.UUID, p customer.UpdatePatch) (customer.Customer, error) {
	v, e := f.Find(ctx, organizationID, id)
	if e != nil {
		return v, e
	}
	if p.Name != nil {
		v.Name = *p.Name
	}
	f.values[organizationID.String()+id.String()] = v
	return v, nil
}
func (f *customerFake) Delete(ctx context.Context, organizationID, id uuid.UUID) error {
	_, e := f.Find(ctx, organizationID, id)
	if e != nil {
		return e
	}
	delete(f.values, organizationID.String()+id.String())
	return nil
}

type dealFake struct{ values map[string]deal.Deal }

func (f *dealFake) List(_ context.Context, organizationID uuid.UUID, _ deal.ListOptions) ([]deal.Deal, int64, error) {
	out := []deal.Deal{}
	for _, v := range f.values {
		if v.OrganizationID == organizationID {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), nil
}
func (f *dealFake) Find(_ context.Context, organizationID, id uuid.UUID) (deal.Deal, error) {
	v, ok := f.values[organizationID.String()+id.String()]
	if !ok {
		return deal.Deal{}, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
	}
	return v, nil
}
func (f *dealFake) Create(_ context.Context, organizationID uuid.UUID, user string, v deal.Deal) (deal.Deal, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = user
	v.CreatedAt = time.Now()
	f.values[organizationID.String()+v.ID.String()] = v
	return v, nil
}
func (f *dealFake) Update(ctx context.Context, organizationID, id uuid.UUID, _ deal.UpdatePatch) (deal.Deal, error) {
	return f.Find(ctx, organizationID, id)
}

type invoiceFake struct{ values map[string]invoice.Invoice }

func (f *invoiceFake) List(_ context.Context, organizationID uuid.UUID, _ invoice.ListOptions) ([]invoice.Invoice, int64, int64, error) {
	out := []invoice.Invoice{}
	for _, v := range f.values {
		if v.OrganizationID == organizationID {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), 0, nil
}
func (f *invoiceFake) Find(_ context.Context, organizationID, id uuid.UUID) (invoice.Invoice, error) {
	v, ok := f.values[organizationID.String()+id.String()]
	if !ok {
		return invoice.Invoice{}, httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
	}
	return v, nil
}
func (f *invoiceFake) Create(_ context.Context, organizationID uuid.UUID, user string, v invoice.Invoice) (invoice.Invoice, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = user
	v.CreatedAt = time.Now()
	f.values[organizationID.String()+v.ID.String()] = v
	return v, nil
}
func (f *invoiceFake) UpdateLocked(ctx context.Context, organizationID, id uuid.UUID, apply func(invoice.Invoice) (invoice.Invoice, error)) (invoice.Invoice, error) {
	v, e := f.Find(ctx, organizationID, id)
	if e != nil {
		return v, e
	}
	v, e = apply(v)
	if e == nil {
		f.values[organizationID.String()+id.String()] = v
	}
	return v, e
}
func (f *invoiceFake) UpdateStatuses(context.Context, uuid.UUID, []uuid.UUID, string, string) ([]invoice.Invoice, error) {
	return nil, nil
}
func (f *invoiceFake) Summary(_ context.Context, organizationID uuid.UUID, _ time.Time, _ *uuid.UUID) (invoice.Summary, error) {
	var summary invoice.Summary
	for _, value := range f.values {
		if value.OrganizationID != organizationID {
			continue
		}
		switch value.Status {
		case "draft":
			summary.DraftCount++
		case "sent":
			summary.OutstandingAmount += value.Amount
		}
	}
	return summary, nil
}

type taskFake struct{ values map[string]task.Task }

func (f *taskFake) List(_ context.Context, organizationID uuid.UUID, _ task.ListOptions) ([]task.Task, int64, error) {
	out := []task.Task{}
	for _, v := range f.values {
		if v.OrganizationID == organizationID {
			out = append(out, v)
		}
	}
	return out, int64(len(out)), nil
}
func (f *taskFake) Create(_ context.Context, organizationID uuid.UUID, user string, v task.Task) (task.Task, error) {
	v.ID = uuid.New()
	v.OrganizationID = organizationID
	v.OwnerUserID = user
	v.CreatedAt = time.Now()
	f.values[organizationID.String()+v.ID.String()] = v
	return v, nil
}
func (f *taskFake) Update(ctx context.Context, organizationID, id uuid.UUID, _ task.UpdatePatch) (task.Task, error) {
	v, ok := f.values[organizationID.String()+id.String()]
	if !ok {
		return task.Task{}, httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	return v, nil
}
func (f *taskFake) Delete(ctx context.Context, organizationID, id uuid.UUID) error {
	if _, ok := f.values[organizationID.String()+id.String()]; !ok {
		return httperror.New(httperror.NotFound, "task_not_found", "Task not found.", nil)
	}
	delete(f.values, organizationID.String()+id.String())
	return nil
}

func TestFourFeatureIsolationAndContract(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := "user-a"
		if r.Header.Get("Cookie") == "session=b" {
			user = "user-b"
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"` + user + `","name":"User","email":"` + user + `@example.com"}`))
	}))
	defer gateway.Close()
	cfg := config.Config{AuthGatewayURL: gateway.URL, AuthGatewayInternalToken: "component-token", AuthGatewayTimeout: time.Second, PublicOrigin: "http://localhost:5173"}
	s := httpserver.New(cfg, nil, nilLogger(), authgateway.NewClient(cfg), componentOrganizationAuthorizer{})
	customers := &customerFake{values: map[string]customer.Customer{}}
	deals := &dealFake{values: map[string]deal.Deal{}}
	invoices := &invoiceFake{values: map[string]invoice.Invoice{}}
	tasks := &taskFake{values: map[string]task.Task{}}
	customerService := customer.NewService(customers)
	s.Register(customer.NewHandler(customerService), deal.NewHandler(deal.NewService(deals, customerService)), invoice.NewHandler(invoice.NewService(invoices, customerService)), task.NewHandler(task.NewService(tasks)))

	create := func(cookie, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Cookie", cookie)
		req.Header.Set("X-Organization-ID", componentOrganizationID.String())
		req.Header.Set("Origin", cfg.PublicOrigin)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		s.Echo.ServeHTTP(res, req)
		return res
	}
	customerRes := create("session=a", "/api/customers", `{"name":"A","email":"a@example.com","phone":"1","status":"active"}`)
	if customerRes.Code != http.StatusCreated {
		t.Fatalf("customer create status=%d", customerRes.Code)
	}
	var customerID string
	for _, v := range customers.values {
		customerID = v.ID.String()
	}
	listB := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/customers", nil)
	req.Header.Set("Cookie", "session=b")
	req.Header.Set("X-Organization-ID", componentOrganizationID.String())
	s.Echo.ServeHTTP(listB, req)
	if listB.Code != http.StatusOK || !strings.Contains(listB.Body.String(), `"name":"A"`) {
		t.Fatalf("shared customer list: status=%d body=%s", listB.Code, listB.Body.String())
	}
	detailB := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/customers/"+customerID, nil)
	req.Header.Set("Cookie", "session=b")
	req.Header.Set("X-Organization-ID", componentOrganizationID.String())
	s.Echo.ServeHTTP(detailB, req)
	if detailB.Code != http.StatusOK {
		t.Fatalf("shared customer detail=%d", detailB.Code)
	}
	dealRes := create("session=b", "/api/deals", `{"title":"cross","customerId":"`+customerID+`","amount":1,"status":"open"}`)
	if dealRes.Code != http.StatusCreated {
		t.Fatalf("shared customer deal=%d body=%s", dealRes.Code, dealRes.Body.String())
	}
	invoiceRes := create("session=a", "/api/invoices", `{"number":"I-1","customerId":"`+customerID+`","amount":1,"issueDate":"2026-01-01","dueDate":"2026-01-02","status":"draft"}`)
	if invoiceRes.Code != http.StatusCreated {
		t.Fatalf("invoice create=%d body=%s", invoiceRes.Code, invoiceRes.Body.String())
	}
	var invoiceID string
	for _, v := range invoices.values {
		invoiceID = v.ID.String()
	}
	patch := httptest.NewRequest(http.MethodPatch, "/api/invoices/"+invoiceID, strings.NewReader(`{"status":"paid"}`))
	patch.Header.Set("Cookie", "session=a")
	patch.Header.Set("X-Organization-ID", componentOrganizationID.String())
	patch.Header.Set("Origin", cfg.PublicOrigin)
	patch.Header.Set("Content-Type", "application/json")
	transition := httptest.NewRecorder()
	s.Echo.ServeHTTP(transition, patch)
	if transition.Code != http.StatusConflict {
		t.Fatalf("invalid invoice transition=%d", transition.Code)
	}
	taskRes := create("session=a", "/api/tasks", `{"title":"A","description":"task","status":"TODO"}`)
	if taskRes.Code != http.StatusCreated {
		t.Fatalf("task create=%d", taskRes.Code)
	}
	var taskID string
	for _, v := range tasks.values {
		taskID = v.ID.String()
	}
	taskPatch := httptest.NewRequest(http.MethodPatch, "/api/tasks/"+taskID, strings.NewReader(`{"status":"DONE"}`))
	taskPatch.Header.Set("Cookie", "session=b")
	taskPatch.Header.Set("X-Organization-ID", componentOrganizationID.String())
	taskPatch.Header.Set("Origin", cfg.PublicOrigin)
	taskPatch.Header.Set("Content-Type", "application/json")
	taskResB := httptest.NewRecorder()
	s.Echo.ServeHTTP(taskResB, taskPatch)
	if taskResB.Code != http.StatusOK {
		t.Fatalf("shared task update=%d", taskResB.Code)
	}
}
