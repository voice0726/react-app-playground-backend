package httpjson

import (
	"errors"
	"io"
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

func TestDecodeAcceptsJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"known":"x"}`))
	r.Header.Set("Content-Type", "application/json")
	var v struct {
		Known string `json:"known"`
	}
	if err := Decode(r, &v); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if v.Known != "x" {
		t.Fatalf("decoded value = %q, want %q", v.Known, "x")
	}
}

func TestDecodeAcceptsJSONMediaTypeParametersAndCase(t *testing.T) {
	for _, contentType := range []string{
		"application/json; charset=utf-8",
		"Application/JSON; charset=utf-8",
	} {
		t.Run(contentType, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"known":"x"}`))
			r.Header.Set("Content-Type", contentType)
			var v struct {
				Known string `json:"known"`
			}
			if err := Decode(r, &v); err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if v.Known != "x" {
				t.Fatalf("decoded value = %q, want %q", v.Known, "x")
			}
		})
	}
}

func TestDecodeRejectsInvalidContentType(t *testing.T) {
	for _, contentType := range []string{"", "text/plain", "application/json; charset"} {
		t.Run(contentType, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"known":"x"}`))
			if contentType != "" {
				r.Header.Set("Content-Type", contentType)
			}
			var v struct {
				Known string `json:"known"`
			}
			if err := Decode(r, &v); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Decode error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func TestDecodePreservesJSONValidationWithMediaTypeParameters(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "oversized", body: `{"known":"x"}` + strings.Repeat(" ", maxBodyBytes)},
		{name: "unknown field", body: `{"known":"x","unknown":1}`},
		{name: "trailing JSON", body: `{"known":"x"}{"known":"y"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json; charset=utf-8")
			var v struct {
				Known string `json:"known"`
			}
			if err := Decode(r, &v); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Decode error = %v, want %v", err, ErrInvalid)
			}
		})
	}
}

func TestDecodeAcceptsBodyAtLimit(t *testing.T) {
	const prefix = `{"known":"`
	const suffix = `"}`
	body := prefix + strings.Repeat("x", maxBodyBytes-len(prefix)-len(suffix)) + suffix
	if len(body) != maxBodyBytes {
		t.Fatalf("body length = %d, want %d", len(body), maxBodyBytes)
	}

	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	var v struct {
		Known string `json:"known"`
	}
	if err := Decode(r, &v); err != nil {
		t.Fatalf("Decode rejected body at limit: %v", err)
	}
	if len(v.Known) != maxBodyBytes-len(prefix)-len(suffix) {
		t.Fatalf("decoded value length = %d", len(v.Known))
	}
}

func TestDecodeRejectsOversizedBodyWithTrailingJSON(t *testing.T) {
	validJSON := `{"known":"x"}`
	body := validJSON + strings.Repeat(" ", maxBodyBytes-len(validJSON)+1) + `{"known":"y"}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	var v struct {
		Known string `json:"known"`
	}
	if err := Decode(r, &v); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Decode error = %v, want %v", err, ErrInvalid)
	}
}

func TestDecodeRejectsBodyOneByteOverLimit(t *testing.T) {
	const validJSON = `{"known":"x"}`
	body := validJSON + strings.Repeat(" ", maxBodyBytes+1-len(validJSON))
	if len(body) != maxBodyBytes+1 {
		t.Fatalf("body length = %d, want %d", len(body), maxBodyBytes+1)
	}

	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	var v struct {
		Known string `json:"known"`
	}
	if err := Decode(r, &v); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Decode error = %v, want %v", err, ErrInvalid)
	}
}

type failingBodyReader struct {
	err error
}

func (r failingBodyReader) Read(p []byte) (int, error) {
	const body = `{"known":"x"}`
	return copy(p, body), r.err
}

func TestDecodeRejectsBodyReadError(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(""))
	r.Body = io.NopCloser(failingBodyReader{err: errors.New("body read failed")})
	r.Header.Set("Content-Type", "application/json")
	var v struct {
		Known string `json:"known"`
	}
	if err := Decode(r, &v); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Decode error = %v, want %v", err, ErrInvalid)
	}
}
