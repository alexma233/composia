package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/store"
)

type taskLogAckState struct {
	mu          sync.Mutex
	confirmedBy map[string]uint64
}

type taskLogUploadSession struct {
	taskID  string
	logPath string
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
		if err := server.bindTaskLogUploadSession(ctx, session, message.GetTaskId()); err != nil {
			return err
		}

		confirmedSeq, err := server.applyTaskLogUpload(session, message)
		if err != nil {
			return err
		}

		if err := stream.Send(taskLogAckResponse(session.taskID, confirmedSeq)); err != nil {
			return err
		}
	}
}

func (server *agentReportServer) bindTaskLogUploadSession(ctx context.Context, session *taskLogUploadSession, taskID string) error {
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
		session.taskID = taskID
		session.logPath = detail.Record.LogPath
		return nil
	}
	if taskID != session.taskID {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("all log events in one stream must use the same task_id"))
	}
	return nil
}

func (server *agentReportServer) applyTaskLogUpload(session *taskLogUploadSession, message *agentv1.UploadTaskLogsRequest) (uint64, error) {
	confirmedSeq := server.confirmedTaskLogSeq(session.taskID)
	switch {
	case message.GetSeq() <= confirmedSeq:
		return confirmedSeq, nil
	case message.GetSeq() != confirmedSeq+1:
		return 0, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("log seq %d is ahead of expected next seq %d", message.GetSeq(), confirmedSeq+1))
	}

	if err := appendTaskLogRaw(session.logPath, message.GetContent()); err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	confirmedSeq = message.GetSeq()
	server.setConfirmedTaskLogSeq(session.taskID, confirmedSeq)
	return confirmedSeq, nil
}

func taskLogAckResponse(taskID string, confirmedSeq uint64) *agentv1.UploadTaskLogsResponse {
	return &agentv1.UploadTaskLogsResponse{TaskId: taskID, LastConfirmedSeq: confirmedSeq}
}

func (server *agentReportServer) confirmedTaskLogSeq(taskID string) uint64 {
	state := server.taskLogState()
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.confirmedBy[taskID]
}

func (server *agentReportServer) setConfirmedTaskLogSeq(taskID string, seq uint64) {
	state := server.taskLogState()
	state.mu.Lock()
	defer state.mu.Unlock()
	state.confirmedBy[taskID] = seq
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
