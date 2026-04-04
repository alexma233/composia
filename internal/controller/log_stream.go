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

func (server *agentReportServer) UploadTaskLogs(ctx context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
	var currentTaskID string
	var currentLogPath string

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
		if currentTaskID == "" {
			if err := ensureTaskNodeMatch(ctx, server.db, message.GetTaskId()); err != nil {
				return err
			}
			detail, err := server.db.GetTask(ctx, message.GetTaskId())
			if err != nil {
				if errors.Is(err, store.ErrTaskNotFound) {
					return connect.NewError(connect.CodeNotFound, err)
				}
				return connect.NewError(connect.CodeInternal, err)
			}
			currentTaskID = message.GetTaskId()
			currentLogPath = detail.Record.LogPath
		} else if message.GetTaskId() != currentTaskID {
			return connect.NewError(connect.CodeInvalidArgument, errors.New("all log events in one stream must use the same task_id"))
		}

		confirmedSeq := server.confirmedTaskLogSeq(currentTaskID)
		switch {
		case message.GetSeq() <= confirmedSeq:
			// Ignore replayed log events after reconnect and repeat the latest ack.
		case message.GetSeq() == confirmedSeq+1:
			if err := appendTaskLogRaw(currentLogPath, message.GetContent()); err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			confirmedSeq = message.GetSeq()
			server.setConfirmedTaskLogSeq(currentTaskID, confirmedSeq)
		default:
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("log seq %d is ahead of expected next seq %d", message.GetSeq(), confirmedSeq+1))
		}

		if err := stream.Send(&agentv1.UploadTaskLogsResponse{TaskId: currentTaskID, LastConfirmedSeq: confirmedSeq}); err != nil {
			return err
		}
	}
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
