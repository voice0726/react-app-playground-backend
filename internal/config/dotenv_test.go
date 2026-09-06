package config

import (
	"os"
	"strings"
	"testing"
)

func unsetForTest(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestDotEnvKeepsExportedValuesAndParsesQuotedValues(t *testing.T) {
	t.Chdir(t.TempDir())
	unsetForTest(t, "DOTENV_FILE_VALUE")
	t.Setenv("DOTENV_EXPORTED_VALUE", "from-shell")
	t.Setenv("DOTENV_EMPTY_VALUE", "")
	content := "DOTENV_FILE_VALUE='value with # and spaces'\nDOTENV_EXPORTED_VALUE=from-file\nDOTENV_EMPTY_VALUE=from-file\n"
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"DOTENV_FILE_VALUE": "value with # and spaces", "DOTENV_EXPORTED_VALUE": "from-shell", "DOTENV_EMPTY_VALUE": ""} {
		if os.Getenv(key) != want {
			t.Fatalf("unexpected value for %s", key)
		}
	}
}

func TestDotEnvIsOptionalButInvalidFilesFailWithoutLeakingValues(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := LoadDotEnv(); err != nil {
		t.Fatalf("missing .env must be optional: %v", err)
	}
	if err := os.WriteFile(".env", []byte("INVALID='private-test-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadDotEnv(); err == nil || strings.Contains(err.Error(), "private-test-value") {
		t.Fatal("invalid .env must fail without exposing its content")
	}
}

func TestLoadReadsAPISettingsFromDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	values := map[string]string{
		"SERVER_ADDR": ":8080", "DATABASE_URL": "postgres://localhost/example",
		"AUTH_GATEWAY_URL": "http://localhost:4000", "AUTH_GATEWAY_INTERNAL_TOKEN": "test-placeholder",
		"PUBLIC_ORIGIN": "http://localhost:5173", "AUTH_GATEWAY_TIMEOUT": "2s",
		"SHUTDOWN_TIMEOUT": "5s", "LOG_LEVEL": "info", "LOG_FORMAT": "text",
	}
	var content strings.Builder
	for key, value := range values {
		unsetForTest(t, key)
		content.WriteString(key + "=" + value + "\n")
	}
	if err := os.WriteFile(".env", []byte(content.String()), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil || cfg.ServerAddr != ":8080" || cfg.AuthGatewayTimeout.Seconds() != 2 {
		t.Fatalf("API settings were not loaded correctly: %v", err)
	}
}
