package controller

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	appnotify "forgejo.alexma.top/alexma233/composia/internal/app/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type taskServer struct {
	db               *store.DB
	cfg              *config.ControllerConfig
	availableNodeIDs map[string]struct{}
	taskQueue        *taskQueueNotifier
	taskResults      *taskResultNotifier
	notifier         *appnotify.Notifier
}

const (
	confirmationDecisionApprove = "approve"
	confirmationDecisionReject  = "reject"
)

func (server *taskServer) ListTasks(ctx context.Context, req *connect.Request[controllerv1.ListTasksRequest]) (*connect.Response[controllerv1.ListTasksResponse], error) {
	if req.Msg == nil {
		req.Msg = &controllerv1.ListTasksRequest{}
	}

	tasks, totalCount, err := server.db.ListTasks(ctx, req.Msg.GetStatus(), req.Msg.GetServiceName(), req.Msg.GetNodeId(), req.Msg.GetType(), req.Msg.GetExcludeStatus(), req.Msg.GetExcludeServiceName(), req.Msg.GetExcludeNodeId(), req.Msg.GetExcludeType(), req.Msg.GetPage(), req.Msg.GetPageSize())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.ListTasksResponse{
		Tasks:      make([]*controllerv1.TaskSummary, 0, len(tasks)),
		TotalCount: totalCount,
	}
	for _, record := range tasks {
		response.Tasks = append(response.Tasks, taskSummaryMessage(record))
	}

	return connect.NewResponse(response), nil
}

func taskSummaryMessage(record store.TaskSummary) *controllerv1.TaskSummary {
	return &controllerv1.TaskSummary{
		TaskId:      record.TaskID,
		Type:        record.Type,
		Status:      record.Status,
		ServiceName: record.ServiceName,
		NodeId:      record.NodeID,
		CreatedAt:   record.CreatedAt,
	}
}

func (server *taskServer) GetTask(ctx context.Context, req *connect.Request[controllerv1.GetTaskRequest]) (*connect.Response[controllerv1.GetTaskResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &controllerv1.GetTaskResponse{
		TaskId:          detail.Record.TaskID,
		Type:            string(detail.Record.Type),
		Source:          string(detail.Record.Source),
		ServiceName:     detail.Record.ServiceName,
		NodeId:          detail.Record.NodeID,
		Status:          string(detail.Record.Status),
		CreatedAt:       detail.Record.CreatedAt.UTC().Format(time.RFC3339),
		StartedAt:       formatNullableTime(detail.Record.StartedAt),
		FinishedAt:      formatNullableTime(detail.Record.FinishedAt),
		RepoRevision:    detail.Record.RepoRevision,
		ErrorSummary:    detail.Record.ErrorSummary,
		LogPath:         detail.Record.LogPath,
		TriggeredBy:     detail.Record.TriggeredBy,
		ResultRevision:  detail.Record.ResultRevision,
		AttemptOfTaskId: detail.Record.AttemptOfTaskID,
		Steps:           make([]*controllerv1.TaskStepSummary, 0, len(detail.Steps)),
	}
	for _, step := range detail.Steps {
		response.Steps = append(response.Steps, &controllerv1.TaskStepSummary{
			StepName:   string(step.StepName),
			Status:     string(step.Status),
			StartedAt:  formatNullableTime(step.StartedAt),
			FinishedAt: formatNullableTime(step.FinishedAt),
		})
	}

	return connect.NewResponse(response), nil
}

func (server *taskServer) TailTaskLogs(ctx context.Context, req *connect.Request[controllerv1.TailTaskLogsRequest], stream *connect.ServerStream[controllerv1.TailTaskLogsResponse]) error {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.LogPath == "" {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("task does not have a log file"))
	}

	var offset int64
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		content, nextOffset, err := readNewLogContent(detail.Record.LogPath, offset)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		offset = nextOffset
		if content != "" {
			if err := stream.Send(&controllerv1.TailTaskLogsResponse{Content: content}); err != nil {
				return err
			}
		}
		refreshedDetail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) {
				return connect.NewError(connect.CodeNotFound, err)
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		detail = refreshedDetail
		if content == "" && isTerminalTaskStatus(detail.Record.Status) {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (server *taskServer) RunTaskAgain(ctx context.Context, req *connect.Request[controllerv1.RunTaskAgainRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.ServiceName == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be rerun without service_name"))
	}

	var rerunType task.Type
	switch detail.Record.Type {
	case task.TypeDeploy, task.TypeUpdate, task.TypeStop, task.TypeRestart, task.TypeBackup:
		rerunType = detail.Record.Type
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task type %q cannot be rerun yet", detail.Record.Type))
	}

	params, err := taskParams(detail.Record.ParamsJSON)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var targetNodeIDs []string
	if detail.Record.NodeID != "" {
		targetNodeIDs = []string{detail.Record.NodeID}
	}
	createdTask, err := createServiceTaskWithOptions(ctx, server.db, server.cfg, server.availableNodeIDs, detail.Record.ServiceName, targetNodeIDs, rerunType, params.DataNames, serviceTaskCreateOptions{AttemptOfTaskID: detail.Record.TaskID, Source: requestTaskSource(req.Header()), ComposeRecreateMode: params.ComposeRecreateMode})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(taskActionResponse(createdTask)), nil
}

func (server *taskServer) ResolveTaskConfirmation(ctx context.Context, req *connect.Request[controllerv1.ResolveTaskConfirmationRequest]) (*connect.Response[controllerv1.TaskActionResponse], error) {
	if req.Msg == nil || req.Msg.GetTaskId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	decision := strings.ToLower(strings.TrimSpace(req.Msg.GetDecision()))
	if decision != confirmationDecisionApprove && decision != confirmationDecisionReject {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("decision must be approve or reject"))
	}

	detail, err := server.db.GetTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if detail.Record.Type != task.TypeMigrate {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task type %q does not support confirmation resolution", detail.Record.Type))
	}
	if detail.Record.Status != task.StatusAwaitingConfirmation {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task %q is not awaiting confirmation", detail.Record.TaskID))
	}

	comment := strings.TrimSpace(req.Msg.GetComment())
	resolvedBy, _ := rpcutil.BearerSubject(ctx)
	resolvedLine := fmt.Sprintf("manual confirmation decision=%s", decision)
	if resolvedBy != "" {
		resolvedLine += fmt.Sprintf(" actor=%s", resolvedBy)
	}
	if comment != "" {
		resolvedLine += fmt.Sprintf(" comment=%q", comment)
	}
	resolvedLine += "\n"
	if err := appendTaskLogRaw(detail.Record.LogPath, resolvedLine); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	finishedAt := time.Now().UTC()
	stepStatus := task.StatusSucceeded
	errorSummary := ""
	if decision == confirmationDecisionReject {
		stepStatus = task.StatusCancelled
		errorSummary = "manual verification rejected"
		if comment != "" {
			errorSummary = fmt.Sprintf("manual verification rejected: %s", comment)
		}
	}
	if err := server.db.UpsertTaskStep(ctx, task.StepRecord{TaskID: detail.Record.TaskID, StepName: task.StepAwaitingConfirmation, Status: stepStatus, StartedAt: findTaskStepStartedAt(detail.Steps, task.StepAwaitingConfirmation), FinishedAt: &finishedAt}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if decision == confirmationDecisionApprove {
		if err := server.db.TransitionTaskStatus(ctx, detail.Record.TaskID, task.StatusAwaitingConfirmation, task.StatusPending, ""); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		detail.Record.Status = task.StatusPending
		notifyTaskQueue(server.taskQueue)
		return connect.NewResponse(taskActionResponse(detail.Record)), nil
	}

	if err := server.db.CompleteTask(ctx, detail.Record.TaskID, task.StatusCancelled, finishedAt, errorSummary); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	detail.Record.Status = task.StatusCancelled
	detail.Record.FinishedAt = &finishedAt
	detail.Record.ErrorSummary = errorSummary
	notifyTaskResult(server.taskResults, detail.Record.TaskID)
	dispatchTaskRecordNotification(server.notifier, corenotify.EventTaskCancelled, detail.Record)
	return connect.NewResponse(taskActionResponse(detail.Record)), nil
}

func formatNullableTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func taskActionResponse(record task.Record) *controllerv1.TaskActionResponse {
	return &controllerv1.TaskActionResponse{
		TaskId:       record.TaskID,
		Status:       string(record.Status),
		RepoRevision: record.RepoRevision,
	}
}

func readNewLogContent(logPath string, offset int64) (string, int64, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return "", offset, fmt.Errorf("open task log %q: %w", logPath, err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return "", offset, fmt.Errorf("stat task log %q: %w", logPath, err)
	}
	if offset > stat.Size() {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", offset, fmt.Errorf("seek task log %q: %w", logPath, err)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return "", offset, fmt.Errorf("read task log %q: %w", logPath, err)
	}
	return string(content), stat.Size(), nil
}

func isTerminalTaskStatus(status task.Status) bool {
	switch status {
	case task.StatusSucceeded, task.StatusFailed, task.StatusCancelled:
		return true
	default:
		return false
	}
}

func requestTaskSource(header http.Header) task.Source {
	switch strings.ToLower(strings.TrimSpace(header.Get("X-Composia-Source"))) {
	case string(task.SourceWeb):
		return task.SourceWeb
	case string(task.SourceOthers):
		return task.SourceOthers
	case string(task.SourceSchedule):
		return task.SourceSchedule
	case string(task.SourceSystem):
		return task.SourceSystem
	default:
		return task.SourceCLI
	}
}

func sourceHeader(source task.Source) http.Header {
	if source == "" {
		return make(http.Header)
	}
	header := make(http.Header)
	header.Set("X-Composia-Source", string(source))
	return header
}

func mustRelativeServiceDir(repoDir, serviceDir string) string {
	relativePath, err := filepath.Rel(repoDir, serviceDir)
	if err != nil {
		return filepath.ToSlash(serviceDir)
	}
	return filepath.ToSlash(relativePath)
}
