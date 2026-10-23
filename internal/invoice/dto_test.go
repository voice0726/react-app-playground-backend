package invoice

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
	"github.com/voice0726/react-app-playground-backend/internal/shared/orgcontext"
)

func TestBulkStatusRequestParse(t *testing.T) {
	id := uuid.NewString()
	maxIDs := make([]string, MaxBulkStatusIDs)
	for index := range maxIDs {
		maxIDs[index] = uuid.NewString()
	}
	tooMany := make([]string, MaxBulkStatusIDs+1)
	for index := range tooMany {
		tooMany[index] = uuid.NewString()
	}
	for _, tc := range []struct {
		name    string
		request BulkStatusRequest
		valid   bool
	}{
		{name: "draft to sent", request: BulkStatusRequest{IDs: []string{id}, From: "draft", To: "sent"}, valid: true},
		{name: "sent to paid", request: BulkStatusRequest{IDs: []string{id}, From: "sent", To: "paid"}, valid: true},
		{name: "draft to paid", request: BulkStatusRequest{IDs: []string{id}, From: "draft", To: "paid"}},
		{name: "cancel", request: BulkStatusRequest{IDs: []string{id}, From: "sent", To: "cancelled"}},
		{name: "same status", request: BulkStatusRequest{IDs: []string{id}, From: "sent", To: "sent"}},
		{name: "empty ids", request: BulkStatusRequest{IDs: []string{}, From: "draft", To: "sent"}},
		{name: "max ids", request: BulkStatusRequest{IDs: maxIDs, From: "draft", To: "sent"}, valid: true},
		{name: "too many ids", request: BulkStatusRequest{IDs: tooMany, From: "draft", To: "sent"}},
		{name: "invalid id", request: BulkStatusRequest{IDs: []string{"not-a-uuid"}, From: "draft", To: "sent"}},
		{name: "duplicate id", request: BulkStatusRequest{IDs: []string{id, id}, From: "draft", To: "sent"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.request.Parse()
			if got := err == nil; got != tc.valid {
				t.Fatalf("valid=%v, want %v (err=%v)", got, tc.valid, err)
			}
		})
	}
}

func TestUpdateStatusesRejectsInvalidRequestBeforeRepository(t *testing.T) {
	repo := &dateValidationRepository{}
	service := NewService(repo, nil)
	_, err := service.UpdateStatuses(context.Background(), orgcontext.Scope{OrganizationID: uuid.New(), UserID: "user-a", Role: "admin"}, BulkStatusRequest{IDs: []string{uuid.NewString()}, From: "draft", To: "paid"})
	httpErr, ok := err.(*httperror.Error)
	if !ok || httpErr.Kind != httperror.Validation {
		t.Fatalf("error=%v, want validation httperror", err)
	}
	if repo.updates != 0 {
		t.Fatalf("updates=%d, want 0", repo.updates)
	}
}
