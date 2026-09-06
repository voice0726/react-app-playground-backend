package dberror

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

func Classify(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503" || pgErr.Code == "23001") {
		return httperror.New(httperror.Conflict, "database_conflict", "The request conflicts with existing data.", err)
	}
	return httperror.New(httperror.Internal, "internal_error", "Internal server error.", err)
}
