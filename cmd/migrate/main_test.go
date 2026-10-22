package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunLoadsDotEnvBeforeValidatingArguments(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATABASE_URL", "")
	if err := os.Unsetenv("DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("DATABASE_URL=postgres://localhost/test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := run(nil)
	if err == nil || !strings.HasPrefix(err.Error(), "usage: migrate") {
		t.Fatalf("expected usage after loading DATABASE_URL without connecting to DB, got %v", err)
	}
}
