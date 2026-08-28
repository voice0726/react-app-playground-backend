package dberror

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voice0726/react-app-playground-backend/internal/shared/httperror"
)

func TestClassifyPostgresConstraintErrorsAsConflict(t *testing.T) {
	for _, code := range []string{"23505", "23503"} {
		t.Run(code, func(t *testing.T) {
			classified := Classify(&pgconn.PgError{Code: code, Message: "constraint detail must not be exposed"})
			var httpErr *httperror.Error
			if !errors.As(classified, &httpErr) || httpErr.Kind != httperror.Conflict {
				t.Fatalf("classified=%v, want conflict", classified)
			}
			if httpErr.Message == "constraint detail must not be exposed" || httpErr.Error() == "" {
				t.Fatal("constraint detail leaked into the public error")
			}
		})
	}
}

func TestClassifyUnknownDatabaseErrorAsInternal(t *testing.T) {
	classified := Classify(errors.New("database failure"))
	var httpErr *httperror.Error
	if !errors.As(classified, &httpErr) || httpErr.Kind != httperror.Internal {
		t.Fatalf("classified=%v, want internal", classified)
	}
}
