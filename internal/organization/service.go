package organization

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) List(ctx context.Context, userID string) ([]Organization, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *Service) Create(ctx context.Context, userID, name string) (Organization, error) {
	if err := validateName(name); err != nil {
		return Organization{}, httperror.New(httperror.Validation, "organization_validation", "Invalid organization.", err)
	}
	return s.repo.Create(ctx, userID, strings.TrimSpace(name))
}

func (s *Service) Role(ctx context.Context, userID string, organizationID uuid.UUID) (string, error) {
	return s.repo.Role(ctx, userID, organizationID)
}

func (s *Service) ListMembers(ctx context.Context, userID string, organizationID uuid.UUID) ([]Member, error) {
	return s.repo.ListMembers(ctx, userID, organizationID)
}

func (s *Service) AddMember(ctx context.Context, actorID string, organizationID uuid.UUID, request MemberRequest) (Member, error) {
	if err := request.Validate(); err != nil {
		return Member{}, httperror.New(httperror.Validation, "member_validation", "Invalid organization member.", err)
	}
	return s.repo.AddMember(ctx, actorID, organizationID, request.UserID, request.Role)
}

func (s *Service) UpdateMemberRole(ctx context.Context, actorID string, organizationID uuid.UUID, userID, role string) (Member, error) {
	request := MemberRequest{UserID: userID, Role: role}
	if err := request.Validate(); err != nil {
		return Member{}, httperror.New(httperror.Validation, "member_validation", "Invalid organization member.", err)
	}
	return s.repo.UpdateMemberRole(ctx, actorID, organizationID, userID, role)
}

func (s *Service) RemoveMember(ctx context.Context, actorID string, organizationID uuid.UUID, userID string) error {
	if strings.TrimSpace(userID) == "" || utf8.RuneCountInString(userID) > maxMemberUserID {
		return httperror.New(httperror.Validation, "member_validation", "Invalid organization member.", nil)
	}
	return s.repo.RemoveMember(ctx, actorID, organizationID, userID)
}
