package organization

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListForUser(context.Context, string) ([]Organization, error)
	Create(context.Context, string, string) (Organization, error)
	Role(context.Context, string, uuid.UUID) (string, error)
	ListMembers(context.Context, string, uuid.UUID) ([]Member, error)
	AddMember(context.Context, string, uuid.UUID, string, string) (Member, error)
	UpdateMemberRole(context.Context, string, uuid.UUID, string, string) (Member, error)
	RemoveMember(context.Context, string, uuid.UUID, string) error
}

type Access interface {
	Role(context.Context, string, uuid.UUID) (string, error)
}
