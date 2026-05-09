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
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestContainerServiceGetContainerLogsStreamsThroughAgentTunnel(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = stream.Close() }()

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

func TestContainerLogTunnelManagerDoesNotBlockWhenAgentQueueIsFull(t *testing.T) {
	t.Parallel()

	manager := newContainerLogTunnelManager()
	tunnel := manager.registerTunnel("main")
	for i := 0; i < cap(tunnel.sendCh); i++ {
		tunnel.sendCh <- &agentv1.OpenContainerLogTunnelResponse{}
	}

	done := make(chan error, 1)
	go func() {
		_, err := manager.openSession("main", "ctr", "", false)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected full tunnel queue to fail")
		}
	case <-time.After(time.Second):
		t.Fatal("openSession blocked on full tunnel queue")
	}
}

func TestContainerLogTunnelManagerOldTunnelUnregisterDoesNotCloseNewSessions(t *testing.T) {
	t.Parallel()

	manager := newContainerLogTunnelManager()
	oldTunnel := manager.registerTunnel("main")
	newTunnel := manager.registerTunnel("main")
	select {
	case _, ok := <-oldTunnel.sendCh:
		if ok {
			t.Fatal("expected old tunnel send channel to be closed")
		}
	default:
		t.Fatal("expected old tunnel send channel to be closed")
	}

	session, err := manager.openSession("main", "ctr", "", false)
	if err != nil {
		t.Fatalf("open session on new tunnel: %v", err)
	}
	select {
	case message := <-newTunnel.sendCh:
		if message.GetSessionId() != session.id {
			t.Fatalf("expected start message for session %q, got %+v", session.id, message)
		}
	default:
		t.Fatal("expected new tunnel to receive session start")
	}

	manager.unregisterTunnel("main", oldTunnel)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.sessions[session.id] == nil {
		t.Fatal("expected new tunnel session to survive old tunnel unregister")
	}
}
