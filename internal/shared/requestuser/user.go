package requestuser

import (
	"context"
	"errors"
)

type AuthenticatedUser struct {
	ID    string
	Name  string
	Email string
	Roles []string
}
type key struct{}

var ErrMissing = errors.New("authenticated user is missing")

func With(ctx context.Context, user AuthenticatedUser) context.Context {
	return context.WithValue(ctx, key{}, user)
}
func From(ctx context.Context) (AuthenticatedUser, error) {
	user, ok := ctx.Value(key{}).(AuthenticatedUser)
	if !ok || user.ID == "" {
		return AuthenticatedUser{}, ErrMissing
	}
	return user, nil
}
