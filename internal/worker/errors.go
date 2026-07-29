package worker

import (
	"errors"

	"go.temporal.io/sdk/temporal"

	"github.com/jdholdren/seymour/internal/seymour"
)

// Unwraps the application error from temporal into a seyerr if possible.
//
// Returns true if the error is convertible to a seymour error.
// Returns false otherwise.
func asSeyerr(err error, seyerr **seymour.Error) bool {
	if err == nil {
		return false
	}

	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		return false
	}
	return appErr.Details(seyerr) == nil
}
