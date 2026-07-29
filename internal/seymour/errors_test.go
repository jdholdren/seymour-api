package seymour_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jdholdren/seymour/internal/seymour"
	"github.com/stretchr/testify/assert"
)

func TestEConstructor(t *testing.T) {
	got := seymour.E(
		"something went wrong",
		seymour.ErrorDetail{Field: "name", Error: "was bad"},
		http.StatusBadRequest,
	)
	want := &seymour.Error{
		Err: errors.New("something went wrong"),
		ErrorDetails: []seymour.ErrorDetail{
			{Field: "name", Error: "was bad"},
		},
		Status: http.StatusBadRequest,
	}

	assert.Equal(t, want, got)
}
