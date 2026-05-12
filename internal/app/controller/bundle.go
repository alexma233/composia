package controller

import (
	"bytes"
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"io"
)

type bundleServer struct {
	db  *store.DB
	cfg *config.ControllerConfig
}

func (server *bundleServer) GetServiceBundle(ctx context.Context, req *connect.Request[agentv1.GetServiceBundleRequest], stream *connect.ServerStream[agentv1.GetServiceBundleResponse]) error {
	if req.Msg.GetTaskId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if err := ensureTaskNodeMatch(ctx, server.db, req.Msg.GetTaskId()); err != nil {
		return err
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	var params serviceTaskParams
	if err := json.Unmarshal([]byte(detail.Record.ParamsJSON), &params); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("decode deploy task params: %w", err))
	}
	requestedServiceDir := params.ServiceDir
	if req.Msg.GetServiceDir() != "" {
		params.ServiceDir = req.Msg.GetServiceDir()
	}
	if params.ServiceDir == "" {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("deploy task is missing service_dir"))
	}
	extraFiles, err := bundleExtraFiles(server.cfg, detail.Record, params, params.ServiceDir == requestedServiceDir)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		pipeWriter.CloseWithError(repo.StreamServiceBundleWithExtras(ctx, server.cfg.RepoDir, detail.Record.RepoRevision, params.ServiceDir, extraFiles, pipeWriter))
	}()
	defer func() { _ = pipeReader.Close() }()

	buffer := make([]byte, 32*1024)
	firstChunk := true
	for {
		count, err := pipeReader.Read(buffer)
		if count > 0 {
			response := &agentv1.GetServiceBundleResponse{Data: bytes.Clone(buffer[:count])}
			if firstChunk {
				response.ServiceName = detail.Record.ServiceName
				response.RepoRevision = detail.Record.RepoRevision
				response.RelativeRoot = params.ServiceDir
				firstChunk = false
			}
			if sendErr := stream.Send(response); sendErr != nil {
				return sendErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, fmt.Errorf("read bundle stream: %w", err))
		}
	}
}
