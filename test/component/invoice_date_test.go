package component

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/invoice"
)

func TestInvoiceDatePatchValidationIsHTTP400(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "issue date after due date", body: `{"issueDate":"2026-01-03"}`},
		{name: "due date before issue date", body: `{"dueDate":"2025-12-31"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			invoices := &invoiceFake{values: map[string]invoice.Invoice{
				componentOrganizationID.String() + id.String(): {ID: id, OrganizationID: componentOrganizationID, OwnerUserID: "u1", IssueDate: mustInvoiceDate("2026-01-01"), DueDate: mustInvoiceDate("2026-01-02"), Status: "draft"},
			}}
			s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
			s.Register(invoice.NewHandler(invoice.NewService(invoices, nil)))
			req := httptest.NewRequest(http.MethodPatch, "/api/invoices/"+id.String(), strings.NewReader(tc.body))
			req.Header.Set("Cookie", "session=component")
			req.Header.Set("X-Organization-ID", componentOrganizationID.String())
			req.Header.Set("Origin", "http://localhost:5173")
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			s.Echo.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s, want 400", res.Code, res.Body.String())
			}
			if invoices.values[componentOrganizationID.String()+id.String()].IssueDate != mustInvoiceDate("2026-01-01") || invoices.values[componentOrganizationID.String()+id.String()].DueDate != mustInvoiceDate("2026-01-02") {
				t.Fatal("invalid date patch changed the invoice")
			}
		})
	}
}

func mustInvoiceDate(value string) time.Time {
	result, _ := time.Parse("2006-01-02", value)
	return result
}
