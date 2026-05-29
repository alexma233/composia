package controller

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func ensureTaskNodeMatch(ctx context.Context, db *store.DB, taskID string) error {
	authenticatedNodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing authenticated node"))
	}
	taskNodeID, err := db.TaskNodeID(ctx, taskID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if taskNodeID != authenticatedNodeID {
		return connect.NewError(connect.CodePermissionDenied, errors.New("task does not belong to authenticated node"))
	}
	return nil
}
