package httperror

import "fmt"

type Kind int

const (
	Validation Kind = iota
	Unauthenticated
	NotFound
	Conflict
	AuthGatewayContract
	AuthGatewayUnavailable
	Internal
	Forbidden
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Err     error
	Details map[string]interface{}
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}
func (e *Error) Unwrap() error { return e.Err }
func New(kind Kind, code, message string, err error) *Error {
	return &Error{Kind: kind, Code: code, Message: message, Err: err}
}

func (e *Error) WithDetails(details map[string]interface{}) *Error {
	e.Details = details
	return e
}
