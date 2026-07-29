package errors_test

import (
	"errors"
	"net/http"
	"testing"

	seyerrs "github.com/jdholdren/seymour/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestEConstructor(t *testing.T) {
	got := seyerrs.E(
		"something went wrong",
		seyerrs.Detail{Field: "name", Error: "was bad"},
		http.StatusBadRequest,
	)
	want := &seyerrs.Error{
		Err: errors.New("something went wrong"),
		Details: []seyerrs.Detail{
			{Field: "name", Error: "was bad"},
		},
		Status: http.StatusBadRequest,
	}

	assert.Equal(t, want, got)
}
