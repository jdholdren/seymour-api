package seymour

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrConflict = E("resource already exists", http.StatusConflict)
	ErrNotFound = E("resource not found", http.StatusNotFound)
)

// Error represents a universal error type between the services.
type Error struct {
	Status       int
	Err          error // The error this wraps
	ErrorDetails []ErrorDetail
}

type ErrorDetail struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type transport struct {
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details"`
	Status  int           `json:"status"`
}

func (s *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(transport{
		Message: s.Err.Error(),
		Details: s.ErrorDetails,
		Status:  s.Status,
	})
}

func (s *Error) UnmarshalJSON(byts []byte) error {
	t := transport{}
	if err := json.Unmarshal(byts, &t); err != nil {
		return err
	}

	s.Err = errors.New(t.Message)
	s.ErrorDetails = t.Details
	s.Status = t.Status
	return nil
}

func E(args ...any) *Error {
	ret := &Error{
		Status: http.StatusInternalServerError,
	}

	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			ret.Err = errors.New(arg)
		case error:
			ret.Err = arg
		case int:
			ret.Status = arg
		case ErrorDetail:
			ret.ErrorDetails = append(ret.ErrorDetails, arg)
		case []ErrorDetail:
			ret.ErrorDetails = append(ret.ErrorDetails, arg...)
		}
	}

	return ret
}
