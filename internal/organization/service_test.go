package organization

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type removeMemberRepository struct {
	Repository
	removed []string
}

func (r *removeMemberRepository) RemoveMember(_ context.Context, _ string, _ uuid.UUID, userID string) error {
	r.removed = append(r.removed, userID)
	return nil
}

func TestRemoveMemberCountsUserIDInCharacters(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userID  string
		removed bool
	}{
		{name: "200 multibyte characters", userID: strings.Repeat("あ", maxMemberUserID), removed: true},
		{name: "201 characters", userID: strings.Repeat("a", maxMemberUserID+1), removed: false},
		{name: "blank", userID: "  ", removed: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &removeMemberRepository{}
			err := NewService(repo).RemoveMember(context.Background(), "admin", uuid.New(), tc.userID)
			if tc.removed {
				if err != nil || len(repo.removed) != 1 {
					t.Fatalf("err=%v removed=%d", err, len(repo.removed))
				}
				return
			}
			var domainErr *httperror.Error
			if !errors.As(err, &domainErr) || domainErr.Kind != httperror.Validation || len(repo.removed) != 0 {
				t.Fatalf("err=%v removed=%d", err, len(repo.removed))
			}
		})
	}
}
