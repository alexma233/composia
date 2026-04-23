package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
)

func TestTaskLogUploaderReconnectsAndResendsUnconfirmedLogs(t *testing.T) {
	t.Parallel()

	server := &logUploadTestServer{}
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewAgentReportServiceHandler(server)
	mux.Handle(path, handler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	client := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL)
	uploader := newTaskLogUploader(client, "task-1")

	if err := uploader.Upload(context.Background(), "one\n"); err != nil {
		t.Fatalf("upload first log: %v", err)
	}
	if err := uploader.Upload(context.Background(), "two\n"); err != nil {
		t.Fatalf("upload second log with reconnect: %v", err)
	}
	if err := uploader.Close(); err != nil {
		t.Fatalf("close uploader: %v", err)
	}

	if server.contents != "one\ntwo\n" {
		t.Fatalf("unexpected uploaded contents %q", server.contents)
	}
	if server.streamCount < 2 {
		t.Fatalf("expected at least 2 log streams, got %d", server.streamCount)
	}
}

func TestTaskLogUploaderTimesOutBlockedAck(t *testing.T) {
	t.Parallel()

	server := &logUploadTestServer{blockAck: true}
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewAgentReportServiceHandler(server)
	mux.Handle(path, handler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	client := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL)
	uploader := newTaskLogUploaderWithTimeout(client, "task-1", 50*time.Millisecond)
	err := uploader.Upload(context.Background(), "one\n")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected upload timeout, got %v", err)
	}
	if err := uploader.Close(); err != nil {
		t.Fatalf("close uploader after timeout: %v", err)
	}
	if server.confirmedSeq != 1 {
		t.Fatalf("expected first send to reach server before timeout, got confirmed seq %d", server.confirmedSeq)
	}
}

type logUploadTestServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
	streamCount      int
	confirmedSeq     uint64
	failedSeqTwoOnce bool
	blockAck         bool
	contents         string
}

func (server *logUploadTestServer) UploadTaskLogs(ctx context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
	server.streamCount++
	for {
		message, err := stream.Receive()
		if err != nil {
			return nil
		}
		if message.GetSeq() == 2 && !server.failedSeqTwoOnce {
			server.failedSeqTwoOnce = true
			return fmt.Errorf("drop stream before acking seq 2")
		}
		if message.GetSeq() == server.confirmedSeq+1 {
			server.confirmedSeq = message.GetSeq()
			server.contents += message.GetContent()
		}
		if server.blockAck {
			<-ctx.Done()
			return ctx.Err()
		}
		if err := stream.Send(&agentv1.UploadTaskLogsResponse{TaskId: message.GetTaskId(), LastConfirmedSeq: server.confirmedSeq}); err != nil {
			return err
		}
	}
}

func (server *logUploadTestServer) Heartbeat(context.Context, *connect.Request[agentv1.HeartbeatRequest]) (*connect.Response[agentv1.HeartbeatResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not used"))
}

func (server *logUploadTestServer) ReportTaskState(context.Context, *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not used"))
}

func (server *logUploadTestServer) ReportTaskStepState(context.Context, *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not used"))
}

func (server *logUploadTestServer) ReportBackupResult(context.Context, *connect.Request[agentv1.ReportBackupResultRequest]) (*connect.Response[agentv1.ReportBackupResultResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not used"))
}

func (server *logUploadTestServer) ReportServiceInstanceStatus(context.Context, *connect.Request[agentv1.ReportServiceInstanceStatusRequest]) (*connect.Response[agentv1.ReportServiceInstanceStatusResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not used"))
}
