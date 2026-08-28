package component

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voice0726/react-app-playground-backend/internal/customer"
	"github.com/voice0726/react-app-playground-backend/internal/shared/dberror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type constraintCustomerRepository struct{}

func (constraintCustomerRepository) List(context.Context, string) ([]customer.Customer, error) {
	return nil, nil
}
func (constraintCustomerRepository) Find(context.Context, string, uuid.UUID) (customer.Customer, error) {
	return customer.Customer{}, httperror.New(httperror.NotFound, "customer_not_found", "Customer not found.", nil)
}
func (constraintCustomerRepository) Create(context.Context, string, customer.Customer) (customer.Customer, error) {
	return customer.Customer{}, nil
}
func (constraintCustomerRepository) Update(context.Context, string, uuid.UUID, customer.UpdatePatch) (customer.Customer, error) {
	return customer.Customer{}, nil
}
func (constraintCustomerRepository) Delete(context.Context, string, uuid.UUID) error {
	return dberror.Classify(&pgconn.PgError{Code: "23503", Message: "referenced customer"})
}

func TestClassifiedCustomerDeleteConstraintIsHTTP409(t *testing.T) {
	s := newServer(t, http.StatusOK, `{"user_id":"u1","name":"User","email":"u@example.com"}`)
	s.Register(customer.NewHandler(customer.NewService(constraintCustomerRepository{})))
	req := httptest.NewRequest(http.MethodDelete, "/api/customers/"+uuid.NewString(), nil)
	req.Header.Set("Cookie", "session=component")
	req.Header.Set("Origin", "http://localhost:5173")
	res := httptest.NewRecorder()
	s.Echo.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", res.Code, res.Body.String())
	}
}
