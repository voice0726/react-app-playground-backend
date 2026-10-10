package httpjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20

var ErrInvalid = errors.New("invalid JSON request")

func Decode(r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil || len(body) > maxBodyBytes {
		return ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrInvalid
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return ErrInvalid
	}
	return nil
}
func Write(w http.ResponseWriter, status int, value interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}
