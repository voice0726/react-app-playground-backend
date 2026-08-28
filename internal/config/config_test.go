package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresAllValuesWithoutLeakingToken(t *testing.T) {
	for _, key := range []string{"SERVER_ADDR", "DATABASE_URL", "AUTH_GATEWAY_URL", "AUTH_GATEWAY_INTERNAL_TOKEN", "PUBLIC_ORIGIN", "AUTH_GATEWAY_TIMEOUT", "SHUTDOWN_TIMEOUT", "LOG_LEVEL", "LOG_FORMAT"} {
		t.Setenv(key, "")
	}
	_, err := Load()
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
