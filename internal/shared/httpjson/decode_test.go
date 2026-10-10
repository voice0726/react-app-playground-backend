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
