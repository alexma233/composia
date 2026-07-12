package controller

import (
	"context"
	"errors"
	"io"
	"sync"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

type taskLogAckState struct {
	mu          sync.Mutex
	confirmedBy map[string]uint64
}

type taskLogUploadSession struct {
	taskID      string
	executionID string
	logPath     string
}

func (server *agentReportServer) UploadTaskLogs(ctx context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
	session := &taskLogUploadSession{}

	for {
		message, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if message.GetTaskId() == "" {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
		}
		if message.GetSeq() == 0 {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("seq must be greater than 0"))
		}
		if err := server.bindTaskLogUploadSession(ctx, session, message.GetTaskId(), message.GetExecutionId()); err != nil {
			return err
		}

		confirmedSeq, err := server.applyTaskLogUpload(ctx, session, message)
		if err != nil {
			return err
		}

		if err := stream.Send(taskLogAckResponse(session.taskID, confirmedSeq)); err != nil {
			return err
		}
	}
}

func (server *agentReportServer) bindTaskLogUploadSession(ctx context.Context, session *taskLogUploadSession, taskID, executionID string) error {
	if session.taskID == "" {
		if err := ensureTaskNodeMatch(ctx, server.db, taskID); err != nil {
			return err
		}
		detail, err := server.db.GetTask(ctx, taskID)
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) {
				return connect.NewError(connect.CodeNotFound, err)
			}
			return connect.NewError(connect.CodeInternal, err)
		}
		if err := server.ensureCurrentTaskExecution(ctx, taskID, executionID); err != nil {
			return err
		}
		session.taskID = taskID
		session.executionID = executionID
		session.logPath = detail.Record.LogPath
		return nil
	}
	if taskID != session.taskID || executionID != session.executionID {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("all log events in one stream must use the same task_id and execution_id"))
	}
	return nil
}

func (server *agentReportServer) applyTaskLogUpload(ctx context.Context, session *taskLogUploadSession, message *agentv1.UploadTaskLogsRequest) (uint64, error) {
	state := server.taskLogState()
	state.mu.Lock()
	defer state.mu.Unlock()
	confirmedSeq, ok := state.confirmedBy[session.taskID]
	if !ok {
		var err error
		confirmedSeq, err = server.db.TaskLogConfirmedSeq(ctx, session.taskID)
		if err != nil {
			return 0, connect.NewError(connect.CodeInternal, err)
		}
		state.confirmedBy[session.taskID] = confirmedSeq
	}
	switch {
	case message.GetSeq() <= confirmedSeq:
		return confirmedSeq, nil
	case message.GetSeq() != confirmedSeq+1:
		return confirmedSeq, nil
	}

	if err := appendTaskLogRaw(session.logPath, message.GetContent()); err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	confirmedSeq = message.GetSeq()
	if err := server.db.SetTaskLogConfirmedSeq(ctx, session.taskID, confirmedSeq); err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	state.confirmedBy[session.taskID] = confirmedSeq
	return confirmedSeq, nil
}

func taskLogAckResponse(taskID string, confirmedSeq uint64) *agentv1.UploadTaskLogsResponse {
	return &agentv1.UploadTaskLogsResponse{TaskId: taskID, LastConfirmedSeq: confirmedSeq}
}

func (server *agentReportServer) resetTaskLogAck(taskID string) {
	state := server.taskLogState()
	state.mu.Lock()
	defer state.mu.Unlock()
	delete(state.confirmedBy, taskID)
}

func (server *agentReportServer) taskLogState() *taskLogAckState {
	if server.logState != nil {
		return server.logState
	}
	server.logState = &taskLogAckState{confirmedBy: make(map[string]uint64)}
	return server.logState
}
