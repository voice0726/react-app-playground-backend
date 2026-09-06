package orgcontext

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Scope struct {
	OrganizationID uuid.UUID
	UserID         string
	Role           string
}

var ErrMissing = errors.New("organization scope is missing")

type key struct{}

func With(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, key{}, scope)
}

func From(ctx context.Context) (Scope, error) {
	scope, ok := ctx.Value(key{}).(Scope)
	if !ok || scope.OrganizationID == uuid.Nil || scope.UserID == "" || scope.Role == "" {
		return Scope{}, ErrMissing
	}
	return scope, nil
}
