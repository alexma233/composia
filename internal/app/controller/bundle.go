package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
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
	nodeID, _ := rpcutil.BearerSubject(ctx)
	if _, err := server.db.ValidateTaskExecution(ctx, req.Msg.GetTaskId(), req.Msg.GetExecutionId(), nodeID); err != nil {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.Status != task.StatusRunning {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task %q is not running", detail.Record.TaskID))
	}

	var params serviceTaskParams
	if err := json.Unmarshal([]byte(detail.Record.ParamsJSON), &params); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("decode deploy task params: %w", err))
	}
	requestedServiceDir := params.ServiceDir
	params.ServiceDir, err = authorizedBundleServiceDir(server.cfg, detail.Record, params, req.Msg.GetServiceDir())
	if err != nil {
		return err
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

func authorizedBundleServiceDir(cfg *config.ControllerConfig, record task.Record, params serviceTaskParams, requested string) (string, error) {
	serviceDir, err := cleanBundleServiceDir(params.ServiceDir)
	if err != nil && requested == "" {
		return "", connect.NewError(connect.CodeFailedPrecondition, errors.New("task is missing a valid service_dir"))
	}
	if requested == "" {
		return serviceDir, nil
	}
	requested, err = cleanBundleServiceDir(requested)
	if err != nil {
		return "", connect.NewError(connect.CodePermissionDenied, err)
	}
	if requested == serviceDir {
		return requested, nil
	}

	switch record.Type {
	case task.TypeCaddySync:
		for _, allowed := range params.ServiceDirs {
			cleaned, cleanErr := cleanBundleServiceDir(allowed)
			if cleanErr == nil && requested == cleaned {
				return requested, nil
			}
		}
	case task.TypeBackup, task.TypeRestore:
		rustic, findErr := repo.FindRusticInfraServiceAtRevision(cfg.RepoDir, record.RepoRevision, configuredNodeIDs(cfg))
		if findErr != nil {
			return "", connect.NewError(connect.CodeInternal, findErr)
		}
		rusticDir, relErr := filepath.Rel(cfg.RepoDir, rustic.Directory)
		if relErr != nil {
			return "", connect.NewError(connect.CodeInternal, relErr)
		}
		if cleaned, cleanErr := cleanBundleServiceDir(rusticDir); cleanErr == nil && requested == cleaned {
			return requested, nil
		}
	default:
	}

	return "", connect.NewError(connect.CodePermissionDenied, fmt.Errorf("service_dir %q is not authorized for task %q", requested, record.TaskID))
}

func cleanBundleServiceDir(value string) (string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	if cleaned == "" || cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("service_dir %q must be a repo-relative directory", value)
	}
	return cleaned, nil
}
