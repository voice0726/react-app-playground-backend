package invoice

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

func TestCanTransition(t *testing.T) {
	for _, tc := range []struct {
		current, next string
		valid         bool
	}{{"draft", "draft", true}, {"draft", "sent", true}, {"draft", "paid", false}, {"sent", "sent", true}, {"sent", "paid", true}, {"paid", "sent", false}, {"cancelled", "paid", false}, {"paid", "cancelled", true}} {
		t.Run(tc.current+"-"+tc.next, func(t *testing.T) {
			if got := CanTransition(tc.current, tc.next) == nil; got != tc.valid {
				t.Fatalf("valid=%v, want %v", got, tc.valid)
			}
		})
	}
}

type dateValidationRepository struct {
	value   Invoice
	updates int
}

func (r *dateValidationRepository) List(context.Context, string) ([]Invoice, error) {
	return []Invoice{r.value}, nil
}
func (r *dateValidationRepository) Find(context.Context, string, uuid.UUID) (Invoice, error) {
	return r.value, nil
}
func (r *dateValidationRepository) Create(context.Context, string, Invoice) (Invoice, error) {
	return Invoice{}, nil
}
func (r *dateValidationRepository) UpdateLocked(_ context.Context, _ string, _ uuid.UUID, apply func(Invoice) (Invoice, error)) (Invoice, error) {
	next, err := apply(r.value)
	if err != nil {
		return r.value, err
	}
	r.value = next
	r.updates++
	return next, nil
}

func TestUpdateRejectsEitherInvoiceDateOrder(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		name  string
		patch UpdateRequest
	}{
		{name: "issue date after due date", patch: UpdateRequest{IssueDate: stringPtr("2026-01-03")}},
		{name: "due date before issue date", patch: UpdateRequest{DueDate: stringPtr("2025-12-31")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &dateValidationRepository{value: Invoice{ID: id, OwnerUserID: "user-a", IssueDate: mustDate("2026-01-01"), DueDate: mustDate("2026-01-02"), Status: "draft"}}
			service := NewService(repo, nil)
			_, err := service.Update(context.Background(), "user-a", id, tc.patch)
			httpErr, ok := err.(*httperror.Error)
			if !ok || httpErr.Kind != httperror.Validation {
				t.Fatalf("error=%v, want validation httperror", err)
			}
			if repo.updates != 0 {
				t.Fatalf("updates=%d, want 0", repo.updates)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
func mustDate(value string) (result time.Time) {
	result, _ = date(value)
	return result
}
