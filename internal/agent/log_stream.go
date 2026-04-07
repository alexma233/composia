package agent

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const logStreamRetryDelay = 200 * time.Millisecond

type taskLogUploader struct {
	client           agentv1connect.AgentReportServiceClient
	taskID           string
	nextSeq          uint64
	lastConfirmedSeq uint64
	pending          []*agentv1.UploadTaskLogsRequest
	stream           *connect.BidiStreamForClient[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]
}

func newTaskLogUploader(client agentv1connect.AgentReportServiceClient, taskID string) *taskLogUploader {
	return &taskLogUploader{
		client:  client,
		taskID:  taskID,
		nextSeq: 1,
	}
}

func (uploader *taskLogUploader) Upload(ctx context.Context, content string) error {
	if content == "" {
		return nil
	}
	uploader.pending = append(uploader.pending, &agentv1.UploadTaskLogsRequest{
		TaskId:  uploader.taskID,
		Seq:     uploader.nextSeq,
		SentAt:  timestamppb.Now(),
		Content: content,
	})
	uploader.nextSeq++
	return uploader.flush(ctx)
}

func (uploader *taskLogUploader) Close() error {
	if uploader.stream == nil {
		return nil
	}
	err := uploader.stream.CloseRequest()
	if closeErr := uploader.stream.CloseResponse(); err == nil {
		err = closeErr
	}
	uploader.stream = nil
	return err
}

func (uploader *taskLogUploader) flush(ctx context.Context) error {
	for len(uploader.pending) > 0 {
		current := uploader.pending[0]
		acked, err := uploader.sendPendingLog(ctx, current)
		if err != nil {
			return err
		}
		if !acked {
			continue
		}
	}
	return nil
}

func (uploader *taskLogUploader) ensureStream(ctx context.Context) error {
	if uploader.stream != nil {
		return nil
	}
	uploader.stream = uploader.client.UploadTaskLogs(ctx)
	return nil
}

func (uploader *taskLogUploader) reconnect(ctx context.Context) error {
	_ = uploader.Close()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(logStreamRetryDelay):
	}
	uploader.stream = uploader.client.UploadTaskLogs(ctx)
	return nil
}

func (uploader *taskLogUploader) sendPendingLog(ctx context.Context, current *agentv1.UploadTaskLogsRequest) (bool, error) {
	if err := uploader.ensureStream(ctx); err != nil {
		return false, err
	}
	if err := uploader.stream.Send(current); err != nil {
		if err := uploader.reconnect(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	ack, err := uploader.stream.Receive()
	if err != nil {
		if err := uploader.reconnect(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := uploader.applyAck(current, ack); err != nil {
		return false, err
	}
	return true, nil
}

func (uploader *taskLogUploader) applyAck(current *agentv1.UploadTaskLogsRequest, ack *agentv1.UploadTaskLogsResponse) error {
	if ack.GetTaskId() != "" && ack.GetTaskId() != uploader.taskID {
		return fmt.Errorf("unexpected log ack task_id %q", ack.GetTaskId())
	}
	if ack.GetLastConfirmedSeq() < uploader.lastConfirmedSeq {
		return fmt.Errorf("log ack moved backwards from %d to %d", uploader.lastConfirmedSeq, ack.GetLastConfirmedSeq())
	}
	uploader.lastConfirmedSeq = ack.GetLastConfirmedSeq()
	uploader.dropConfirmed()
	if uploader.lastConfirmedSeq < current.GetSeq() {
		return fmt.Errorf("controller acked seq %d while waiting for %d", uploader.lastConfirmedSeq, current.GetSeq())
	}
	return nil
}

func (uploader *taskLogUploader) dropConfirmed() {
	keep := uploader.pending[:0]
	for _, entry := range uploader.pending {
		if entry.GetSeq() > uploader.lastConfirmedSeq {
			keep = append(keep, entry)
		}
	}
	uploader.pending = keep
}
