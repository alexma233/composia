package agent

import (
	"context"
	"errors"
	"io"
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

	receivedSeq := make(chan uint64, 1)
	server := &logUploadTestServer{blockAck: true, receivedSeq: receivedSeq}
	mux := http.NewServeMux()
	path, handler := agentv1connect.NewAgentReportServiceHandler(server)
	mux.Handle(path, handler)
	httpServer := httptest.NewUnstartedServer(mux)
	httpServer.EnableHTTP2 = true
	httpServer.StartTLS()
	defer httpServer.Close()

	client := agentv1connect.NewAgentReportServiceClient(httpServer.Client(), httpServer.URL)
	uploader := newTaskLogUploaderWithTimeout(client, "task-1", 5*time.Second)
	errCh := make(chan error, 1)
	go func() {
		errCh <- uploader.Upload(context.Background(), "one\n")
	}()

	select {
	case seq := <-receivedSeq:
		if seq != 1 {
			t.Fatalf("expected server to receive seq 1, got %d", seq)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for first send to reach server")
	}

	var err error
	select {
	case err = <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for blocked ack timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected upload timeout, got %v", err)
	}
	if len(uploader.pending) != 0 {
		t.Fatalf("expected failed best-effort logs to be discarded, got %d pending entries", len(uploader.pending))
	}
	if err := uploader.Close(); err != nil {
		t.Fatalf("close uploader after timeout: %v", err)
	}
	if server.confirmedSeq != 1 {
		t.Fatalf("expected first send to reach server before timeout, got confirmed seq %d", server.confirmedSeq)
	}
}

func TestTaskLogUploaderRenegotiatesControllerSequence(t *testing.T) {
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
	uploader.lastConfirmedSeq = 5
	uploader.nextSeq = 6
	if err := uploader.Upload(context.Background(), "after restart\n"); err != nil {
		t.Fatal(err)
	}
	if err := uploader.Close(); err != nil {
		t.Fatal(err)
	}
	if server.contents != "after restart\n" || server.confirmedSeq != 1 || uploader.lastConfirmedSeq != 1 {
		t.Fatalf("sequence was not renegotiated: server_seq=%d agent_seq=%d content=%q", server.confirmedSeq, uploader.lastConfirmedSeq, server.contents)
	}
}

type logUploadTestServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
	streamCount      int
	confirmedSeq     uint64
	failedSeqTwoOnce bool
	blockAck         bool
	receivedSeq      chan uint64
	contents         string
}

func (server *logUploadTestServer) UploadTaskLogs(ctx context.Context, stream *connect.BidiStream[agentv1.UploadTaskLogsRequest, agentv1.UploadTaskLogsResponse]) error {
	server.streamCount++
	for {
		message, err := stream.Receive()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return err
			}
			return nil
		}
		if message.GetSeq() == 2 && !server.failedSeqTwoOnce {
			server.failedSeqTwoOnce = true
			return errors.New("drop stream before acking seq 2")
		}
		if message.GetSeq() == server.confirmedSeq+1 {
			server.confirmedSeq = message.GetSeq()
			server.contents += message.GetContent()
		}
		if server.receivedSeq != nil {
			select {
			case server.receivedSeq <- message.GetSeq():
			default:
			}
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *logUploadTestServer) ReportTaskState(context.Context, *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *logUploadTestServer) ReportTaskStepState(context.Context, *connect.Request[agentv1.ReportTaskStepStateRequest]) (*connect.Response[agentv1.ReportTaskStepStateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *logUploadTestServer) ReportBackupResult(context.Context, *connect.Request[agentv1.ReportBackupResultRequest]) (*connect.Response[agentv1.ReportBackupResultResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (server *logUploadTestServer) ReportServiceInstanceStatus(context.Context, *connect.Request[agentv1.ReportServiceInstanceStatusRequest]) (*connect.Response[agentv1.ReportServiceInstanceStatusResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}
