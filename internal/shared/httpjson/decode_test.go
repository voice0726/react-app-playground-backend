package httpjson

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"known":"x","unknown":1}`, `{"known":"x"}{"known":"y"}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var v struct {
			Known string `json:"known"`
		}
		if Decode(r, &v) == nil {
			t.Fatalf("Decode accepted %s", body)
		}
	}
}
