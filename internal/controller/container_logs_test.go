package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
)

func TestContainerServiceGetContainerLogsStreamsThroughAgentTunnel(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: "main", HeartbeatAt: time.Date(2026, 4, 18, 9, 59, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	logManager := newContainerLogTunnelManager()
	tunnel := logManager.registerTunnel("main")
	defer logManager.unregisterTunnel("main", tunnel)

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "web-admin", nil
	})

	path, handler := controllerv1connect.NewContainerServiceHandler(
		&containerServer{
			db:         db,
			cfg:        &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}}},
			logManager: logManager,
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewContainerServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	agentErrCh := make(chan error, 1)
	go func() {
		message, ok := <-tunnel.sendCh
		if !ok {
			agentErrCh <- fmt.Errorf("container log tunnel closed before start")
			return
		}
		if message.GetKind() != containerLogKindStart {
			agentErrCh <- fmt.Errorf("expected start message, got %q", message.GetKind())
			return
		}
		if message.GetContainerId() != "ctr" {
			agentErrCh <- fmt.Errorf("expected container id ctr, got %q", message.GetContainerId())
			return
		}
		if message.GetTail() != "2" {
			agentErrCh <- fmt.Errorf("expected tail 2, got %q", message.GetTail())
			return
		}
		if message.GetTimestamps() {
			agentErrCh <- fmt.Errorf("expected timestamps to be disabled")
			return
		}

		logManager.deliverFromAgent(&agentv1.OpenContainerLogTunnelRequest{SessionId: message.GetSessionId(), Kind: containerLogKindChunk, Content: "hello\n"})
		logManager.deliverFromAgent(&agentv1.OpenContainerLogTunnelRequest{SessionId: message.GetSessionId(), Kind: containerLogKindChunk, Content: "world\n"})
		logManager.deliverFromAgent(&agentv1.OpenContainerLogTunnelRequest{SessionId: message.GetSessionId(), Kind: containerLogKindClosed})
		agentErrCh <- nil
	}()

	stream, err := client.GetContainerLogs(context.Background(), connect.NewRequest(&controllerv1.GetContainerLogsRequest{NodeId: "main", ContainerId: "ctr", Tail: "2"}))
	if err != nil {
		t.Fatalf("get container logs: %v", err)
	}
	defer stream.Close()

	if !stream.Receive() {
		t.Fatalf("expected first log chunk, got err=%v", stream.Err())
	}
	if stream.Msg().GetContent() != "hello\n" {
		t.Fatalf("unexpected first log chunk %q", stream.Msg().GetContent())
	}

	if !stream.Receive() {
		t.Fatalf("expected second log chunk, got err=%v", stream.Err())
	}
	if stream.Msg().GetContent() != "world\n" {
		t.Fatalf("unexpected second log chunk %q", stream.Msg().GetContent())
	}

	if stream.Receive() {
		t.Fatal("expected stream to close after agent closed the session")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("expected container log stream to close cleanly, got %v", err)
	}

	if err := <-agentErrCh; err != nil {
		t.Fatal(err)
	}
}
