package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var ErrInvalid = errors.New("invalid JSON request")

func Decode(r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return ErrInvalid
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
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
