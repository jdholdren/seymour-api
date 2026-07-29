package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// EnsureDefaultNamespace tries to make sure that the namespace
// that this app uses is existent.
//
// Returns an error when the namespace cannot be created or described.
func EnsureDefaultNamespace(ctx context.Context, cli workflowservice.WorkflowServiceClient) error {
	// First see if it exists
	_, err := cli.RegisterNamespace(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace:                        "default",
		WorkflowExecutionRetentionPeriod: durationpb.New(72 * time.Hour),
	})
	// Handle conflict
	var allreadyErr *serviceerror.NamespaceAlreadyExists
	if errors.As(err, &allreadyErr) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("error registering default namespace: %s", err)
	}

	return nil
}
