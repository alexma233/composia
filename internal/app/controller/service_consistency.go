package controller

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func (server *agentReportServer) ReportServiceConsistencyCheck(ctx context.Context, req *connect.Request[agentv1.ReportServiceConsistencyCheckRequest]) (*connect.Response[agentv1.ReportServiceConsistencyCheckResponse], error) {
	nodeID, err := validateTaskExecutionRequest(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId())
	if err != nil {
		return nil, err
	}
	files, compose := req.Msg.GetFiles(), req.Msg.GetCompose()
	if !store.IsValidConsistencyStatus(files.GetStatus()) || !store.IsValidConsistencyStatus(compose.GetStatus()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("files and compose require valid consistency statuses"))
	}
	err = server.db.RecordServiceConsistencyCheck(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId(), nodeID,
		store.ServiceConsistencyOutcome{Status: files.GetStatus(), Reasons: files.GetReasons()},
		store.ServiceConsistencyOutcome{Status: compose.GetStatus(), Reasons: compose.GetReasons()})
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&agentv1.ReportServiceConsistencyCheckResponse{}), nil
}

func serviceConsistencyMessage(snapshot *store.ServiceConsistencyCheck) *agentv1.ServiceConsistencyCheck {
	if snapshot == nil {
		return &agentv1.ServiceConsistencyCheck{Files: &agentv1.ServiceConsistencyOutcome{Status: store.ConsistencyUnknown}, Compose: &agentv1.ServiceConsistencyOutcome{Status: store.ConsistencyUnknown}}
	}
	return &agentv1.ServiceConsistencyCheck{
		Files:     &agentv1.ServiceConsistencyOutcome{Status: snapshot.Files.Status, Reasons: snapshot.Files.Reasons},
		Compose:   &agentv1.ServiceConsistencyOutcome{Status: snapshot.Compose.Status, Reasons: snapshot.Compose.Reasons},
		CheckedAt: snapshot.CheckedAt, RepoRevision: snapshot.RepoRevision, TaskId: snapshot.TaskID,
	}
}
