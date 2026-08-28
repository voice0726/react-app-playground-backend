package component

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/deal"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type constraintDealRepository struct{}

func (constraintDealRepository) List(context.Context, string) ([]deal.Deal, error) { return nil, nil }
func (constraintDealRepository) Find(context.Context, string, uuid.UUID) (deal.Deal, error) {
	return deal.Deal{}, httperror.New(httperror.NotFound, "deal_not_found", "Deal not found.", nil)
}
func (constraintDealRepository) Create(context.Context, string, deal.Deal) (deal.Deal, error) {
	return deal.Deal{}, dberror.Classify(&pgconn.PgError{Code: "23503", Message: "cross-owner customer foreign key"})
}
func (constraintDealRepository) Update(context.Context, string, uuid.UUID, deal.UpdatePatch) (deal.Deal, error) {
	return deal.Deal{}, nil
}

type constraintInvoiceRepository struct{}

func (constraintInvoiceRepository) List(context.Context, string) ([]invoice.Invoice, error) {
	return nil, nil
}
func (constraintInvoiceRepository) Find(context.Context, string, uuid.UUID) (invoice.Invoice, error) {
	return invoice.Invoice{}, httperror.New(httperror.NotFound, "invoice_not_found", "Invoice not found.", nil)
}
func (constraintInvoiceRepository) Create(context.Context, string, invoice.Invoice) (invoice.Invoice, error) {
	return invoice.Invoice{}, dberror.Classify(&pgconn.PgError{Code: "23505", Message: "owner-scoped invoice number"})
}
func (constraintInvoiceRepository) UpdateLocked(context.Context, string, uuid.UUID, func(invoice.Invoice) (invoice.Invoice, error)) (invoice.Invoice, error) {
	return invoice.Invoice{}, nil
}

func TestDealForeignKeyAndInvoiceDuplicateConstraintsAreHTTP409(t *testing.T) {
	customerID := uuid.New()
	customers := &customerFake{values: map[string]customer.Customer{
		"u1" + customerID.String(): {ID: customerID, OwnerUserID: "u1", Name: "Customer", Email: "customer@example.com", Phone: "1", Status: "active"},
	}}
	s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
	s.Register(
		deal.NewHandler(deal.NewService(constraintDealRepository{}, customers)),
		invoice.NewHandler(invoice.NewService(constraintInvoiceRepository{}, customers)),
	)
	for _, tc := range []struct {
		name, path, body string
	}{
		{name: "deal composite foreign key", path: "/api/deals", body: `{"title":"Deal","customerId":"` + customerID.String() + `","amount":1,"status":"open"}`},
		{name: "invoice number duplicate", path: "/api/invoices", body: `{"number":"I-1","customerId":"` + customerID.String() + `","amount":1,"issueDate":"2026-01-01","dueDate":"2026-01-02","status":"draft"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Cookie", "session=component")
			req.Header.Set("Origin", "http://localhost:5173")
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			s.Echo.ServeHTTP(res, req)
			if res.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s, want 409", res.Code, res.Body.String())
			}
		})
	}
}
