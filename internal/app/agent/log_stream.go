package agent

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"github.com/cenkalti/backoff/v6"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	logStreamInitialRetryDelay = 200 * time.Millisecond
	logStreamMaxRetryDelay     = 5 * time.Second
)

type taskLogUploader struct {
	client           agentv1connect.AgentReportServiceClient
	taskID           string
	executionID      string
	timeout          time.Duration
	nextSeq          uint64
	lastConfirmedSeq uint64
	pending          []*agentv1.UploadTaskLogsRequest
	stream           *connect.BidiStreamForClient[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]
	retryBackOff     *backoff.ExponentialBackOff
}

func newTaskLogUploader(client agentv1connect.AgentReportServiceClient, taskID string, executionID ...string) *taskLogUploader {
	uploader := newTaskLogUploaderWithTimeout(client, taskID, taskReportTimeout)
	if len(executionID) > 0 {
		uploader.executionID = executionID[0]
	}
	return uploader
}

func newTaskLogUploaderWithTimeout(client agentv1connect.AgentReportServiceClient, taskID string, timeout time.Duration) *taskLogUploader {
	return &taskLogUploader{
		client:       client,
		taskID:       taskID,
		timeout:      timeout,
		nextSeq:      1,
		retryBackOff: newLogStreamBackOff(),
	}
}

func newLogStreamBackOff() *backoff.ExponentialBackOff {
	retryBackOff := backoff.NewExponentialBackOff()
	retryBackOff.InitialInterval = logStreamInitialRetryDelay
	retryBackOff.MaxInterval = logStreamMaxRetryDelay
	retryBackOff.Reset()
	return retryBackOff
}

func (uploader *taskLogUploader) Upload(ctx context.Context, content string) error {
	if content == "" {
		return nil
	}
	uploadCtx := ctx
	var cancel context.CancelFunc
	if uploader.timeout > 0 {
		uploadCtx, cancel = context.WithTimeout(ctx, uploader.timeout)
		defer cancel()
	}
	uploader.pending = append(uploader.pending, &agentv1.UploadTaskLogsRequest{
		TaskId:      uploader.taskID,
		ExecutionId: uploader.executionID,
		Seq:         uploader.nextSeq,
		SentAt:      timestamppb.Now(),
		Content:     content,
	})
	uploader.nextSeq++
	if err := uploader.flush(uploadCtx); err != nil {
		uploader.abandonPending()
		return err
	}
	return nil
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
		_, err := uploader.sendPendingLog(ctx, current)
		if err != nil {
			return err
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
	delay := uploader.retryBackOff.NextBackOff()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
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
	confirmed, err := uploader.applyAck(current, ack)
	if err != nil {
		return false, err
	}
	uploader.retryBackOff.Reset()
	return confirmed, nil
}

func (uploader *taskLogUploader) applyAck(current *agentv1.UploadTaskLogsRequest, ack *agentv1.UploadTaskLogsResponse) (bool, error) {
	if ack.GetTaskId() != "" && ack.GetTaskId() != uploader.taskID {
		return false, fmt.Errorf("unexpected log ack task_id %q", ack.GetTaskId())
	}
	if ack.GetLastConfirmedSeq() < uploader.lastConfirmedSeq {
		uploader.lastConfirmedSeq = ack.GetLastConfirmedSeq()
		for index, entry := range uploader.pending {
			entry.Seq = uploader.lastConfirmedSeq + uint64(index) + 1
		}
		uploader.nextSeq = uploader.lastConfirmedSeq + uint64(len(uploader.pending)) + 1
		return false, nil
	}
	uploader.lastConfirmedSeq = ack.GetLastConfirmedSeq()
	uploader.dropConfirmed()
	if uploader.lastConfirmedSeq < current.GetSeq() {
		return false, nil
	}
	return true, nil
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

func (uploader *taskLogUploader) abandonPending() {
	_ = uploader.Close()
	uploader.pending = nil
	uploader.nextSeq = uploader.lastConfirmedSeq + 1
}
