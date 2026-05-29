package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"github.com/gorilla/websocket"
)

func TestContainerExecRequiresWebOriginHeader(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, controllerTestHeartbeat("main")); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	execManager := newExecTunnelManager()
	execManager.registerTunnel("main")

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "web-admin", nil
	})

	path, handler := controllerv1connect.NewDockerCommandServiceHandler(
		&dockerCommandServer{
			db:          db,
			cfg:         &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}}},
			execManager: execManager,
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewDockerCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	_, err := client.OpenContainerExec(ctx, connect.NewRequest(&controllerv1.OpenContainerExecRequest{NodeId: "main", ContainerId: "ctr"}))
	if err == nil {
		t.Fatal("expected missing origin header to fail")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if !strings.Contains(err.Error(), execWebOriginHeader) {
		t.Fatalf("expected error to mention %s, got %v", execWebOriginHeader, err)
	}
}

func TestContainerExecWebsocketRequiresAllowedOriginAndOneTimeToken(t *testing.T) {
	t.Parallel()

	db := openControllerTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main"}); err != nil {
		t.Fatalf("sync configured nodes: %v", err)
	}
	if err := db.RecordHeartbeat(ctx, controllerTestHeartbeat("main")); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}

	execManager := newExecTunnelManager()
	execManager.registerTunnel("main")

	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "web-admin", nil
	})

	path, handler := controllerv1connect.NewDockerCommandServiceHandler(
		&dockerCommandServer{
			db:          db,
			cfg:         &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}}},
			execManager: execManager,
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc(rpcutil.ControllerExecWSPath, execManager.handleWebsocket)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewDockerCommandServiceClient(
		httpServer.Client(),
		httpServer.URL,
		connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor("access-token")),
	)

	req := connect.NewRequest(&controllerv1.OpenContainerExecRequest{NodeId: "main", ContainerId: "ctr"})
	req.Header().Set(execWebOriginHeader, "https://web.example.test")
	response, err := client.OpenContainerExec(ctx, req)
	if err != nil {
		t.Fatalf("open exec session: %v", err)
	}
	if strings.Contains(response.Msg.GetWebsocketPath(), response.Msg.GetSessionId()) {
		t.Fatalf("websocket path must not expose session id: %q", response.Msg.GetWebsocketPath())
	}
	if !strings.HasPrefix(response.Msg.GetWebsocketPath(), rpcutil.ControllerExecWSPath) {
		t.Fatalf("websocket path = %q, want prefix %q", response.Msg.GetWebsocketPath(), rpcutil.ControllerExecWSPath)
	}

	wsURL := websocketURL(httpServer.URL, response.Msg.GetWebsocketPath())
	biddenHeader := http.Header{}
	biddenHeader.Set("Origin", "https://evil.example.test")
	if conn, dialResponse, err := websocket.DefaultDialer.Dial(wsURL, biddenHeader); err == nil {
		_ = conn.Close()
		t.Fatal("expected websocket origin mismatch to fail")
	} else if dialResponse != nil && dialResponse.Body != nil {
		_ = dialResponse.Body.Close()
	}

	allowedHeader := http.Header{}
	allowedHeader.Set("Origin", "https://web.example.test")
	conn, dialResponse, err := websocket.DefaultDialer.Dial(wsURL, allowedHeader)
	if dialResponse != nil && dialResponse.Body != nil {
		defer func() { _ = dialResponse.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("dial websocket with allowed origin: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var ready execWSEvent
	if err := conn.ReadJSON(&ready); err != nil {
		t.Fatalf("read ready event: %v", err)
	}
	if ready.Type != execKindReady {
		t.Fatalf("expected ready event, got %+v", ready)
	}
	if ready.Session != response.Msg.GetSessionId() {
		t.Fatalf("expected ready session %q, got %q", response.Msg.GetSessionId(), ready.Session)
	}

	if extraConn, dialResponse, err := websocket.DefaultDialer.Dial(wsURL, allowedHeader); err == nil {
		_ = extraConn.Close()
		t.Fatal("expected attach token to be single-use")
	} else if dialResponse != nil && dialResponse.Body != nil {
		_ = dialResponse.Body.Close()
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
}

func TestExecTunnelManagerExpiresUnusedAttachToken(t *testing.T) {
	t.Parallel()

	manager := newExecTunnelManager()
	manager.registerTunnel("main")

	session, err := manager.openSession("main", "ctr", []string{"/bin/sh"}, 24, 80, "https://web.example.test", "web-admin")
	if err != nil {
		t.Fatalf("open session: %v", err)
	}

	manager.mu.Lock()
	session.attachExpiresAt = time.Now().UTC().Add(-time.Second)
	manager.mu.Unlock()

	if _, err := manager.reserveSessionAttach(session.attachToken); err == nil {
		t.Fatal("expected expired attach token to fail")
	}
	if got := manager.lookupSession(session.id); got != nil {
		t.Fatalf("expected expired session to be removed, got %+v", got)
	}
}

func TestExecTunnelManagerDoesNotBlockWhenAgentQueueIsFull(t *testing.T) {
	t.Parallel()

	manager := newExecTunnelManager()
	tunnel := manager.registerTunnel("main")
	for range cap(tunnel.sendCh) {
		tunnel.sendCh <- &agentv1.OpenExecTunnelResponse{}
	}

	done := make(chan error, 1)
	go func() {
		_, err := manager.openSession("main", "ctr", []string{"/bin/sh"}, 24, 80, "https://web.example.test", "web-admin")
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

func TestExecTunnelManagerOldTunnelUnregisterDoesNotCloseNewSessions(t *testing.T) {
	t.Parallel()

	manager := newExecTunnelManager()
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

	session, err := manager.openSession("main", "ctr", []string{"/bin/sh"}, 24, 80, "https://web.example.test", "web-admin")
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
	if got := manager.lookupSession(session.id); got == nil {
		t.Fatal("expected new tunnel session to survive old tunnel unregister")
	}
}

func websocketURL(baseURL, path string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + path
}

func controllerTestHeartbeat(nodeID string) store.NodeHeartbeat {
	return store.NodeHeartbeat{NodeID: nodeID, HeartbeatAt: time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)}
}
