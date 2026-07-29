package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error represents a universal error type between the services.
type Error struct {
	Status  int
	Err     error // The error this wraps
	Details []Detail
}

type Detail struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

func (e *Error) Error() string {
	return e.Err.Error()
}

type transport struct {
	Message string   `json:"message"`
	Details []Detail `json:"details"`
	Status  int      `json:"status"`
}

func (s *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(transport{
		Message: s.Err.Error(),
		Details: s.Details,
		Status:  s.Status,
	})
}

func (s *Error) UnmarshalJSON(byts []byte) error {
	t := transport{}
	if err := json.Unmarshal(byts, &t); err != nil {
		return err
	}

	s.Err = errors.New(t.Message)
	s.Details = t.Details
	s.Status = t.Status
	return nil
}

func E(args ...any) *Error {
	ret := &Error{
		Status:  http.StatusInternalServerError,
		Err:     nil,
		Details: nil,
	}

	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			ret.Err = errors.New(arg)
		case error:
			ret.Err = arg
		case int:
			ret.Status = arg
		case Detail:
			ret.Details = append(ret.Details, arg)
		case []Detail:
			ret.Details = append(ret.Details, arg...)
		}
	}

	return ret
}
