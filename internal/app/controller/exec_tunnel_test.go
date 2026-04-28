package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
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

	path, handler := controllerv1connect.NewContainerServiceHandler(
		&containerServer{
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

	client := controllerv1connect.NewContainerServiceClient(
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

	path, handler := controllerv1connect.NewContainerServiceHandler(
		&containerServer{
			db:          db,
			cfg:         &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}}},
			execManager: execManager,
		},
		connect.WithInterceptors(interceptor),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/ws/container-exec/", execManager.handleWebsocket)
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := controllerv1connect.NewContainerServiceClient(
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

	wsURL := websocketURL(httpServer.URL, response.Msg.GetWebsocketPath())
	biddenHeader := http.Header{}
	biddenHeader.Set("Origin", "https://evil.example.test")
	if _, _, err := websocket.DefaultDialer.Dial(wsURL, biddenHeader); err == nil {
		t.Fatal("expected websocket origin mismatch to fail")
	}

	allowedHeader := http.Header{}
	allowedHeader.Set("Origin", "https://web.example.test")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, allowedHeader)
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

	if _, _, err := websocket.DefaultDialer.Dial(wsURL, allowedHeader); err == nil {
		t.Fatal("expected attach token to be single-use")
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

func websocketURL(baseURL, path string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + path
}

func controllerTestHeartbeat(nodeID string) store.NodeHeartbeat {
	return store.NodeHeartbeat{NodeID: nodeID, HeartbeatAt: time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)}
}
