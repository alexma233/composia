package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/config"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/store"
)

func TestNormalizeStreamedContainerLogChunkStripsTimestamps(t *testing.T) {
	t.Parallel()

	content, lastTimestamp, ok := normalizeStreamedContainerLogChunk("2026-04-18T10:00:00Z hello\n2026-04-18T10:00:01Z world\n", false)
	if !ok {
		t.Fatal("expected timestamped log chunk to report a timestamp")
	}
	if content != "hello\nworld\n" {
		t.Fatalf("unexpected stripped content %q", content)
	}
	if want := time.Date(2026, 4, 18, 10, 0, 1, 0, time.UTC); !lastTimestamp.Equal(want) {
		t.Fatalf("expected last timestamp %s, got %s", want.Format(time.RFC3339Nano), lastTimestamp.Format(time.RFC3339Nano))
	}

	preserved, _, ok := normalizeStreamedContainerLogChunk("2026-04-18T10:00:00Z hello\n", true)
	if !ok {
		t.Fatal("expected preserved chunk to report a timestamp")
	}
	if preserved != "2026-04-18T10:00:00Z hello\n" {
		t.Fatalf("unexpected preserved content %q", preserved)
	}
}

func TestContainerServiceGetContainerLogsStreamsIncrementalContent(t *testing.T) {
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

	broker := newDockerQueryBroker()
	interceptor := rpcutil.NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "access-token" {
			return "", assertError("unexpected token")
		}
		return "test-client", nil
	})

	path, handler := controllerv1connect.NewContainerServiceHandler(
		&containerServer{
			db:            db,
			cfg:           &config.ControllerConfig{Nodes: []config.NodeConfig{{ID: "main"}}},
			dockerQueries: broker,
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
		query, err := waitForDockerQueryForTest(broker, "main", 2*time.Second)
		if err != nil {
			agentErrCh <- err
			return
		}
		if query.Tail != "2" {
			agentErrCh <- fmt.Errorf("expected initial tail 2, got %q", query.Tail)
			return
		}
		if !query.Timestamps {
			agentErrCh <- fmt.Errorf("expected controller to request timestamped logs")
			return
		}
		if query.Since != "" {
			agentErrCh <- fmt.Errorf("expected initial since to be empty, got %q", query.Since)
			return
		}
		if err := broker.StoreResult(dockerAgentQueryResult{
			QueryID:     query.QueryID,
			NodeID:      "main",
			PayloadJSON: mustMarshalDockerListResultForTest(dockerListResult{Content: "2026-04-18T10:00:00Z hello\n"}),
		}); err != nil {
			agentErrCh <- fmt.Errorf("store first docker query result: %w", err)
			return
		}

		query, err = waitForDockerQueryForTest(broker, "main", 3*time.Second)
		if err != nil {
			agentErrCh <- err
			return
		}
		if query.Tail != "" {
			agentErrCh <- fmt.Errorf("expected follow-up tail to be empty, got %q", query.Tail)
			return
		}
		wantSince := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC).Add(time.Nanosecond).Format(time.RFC3339Nano)
		if query.Since != wantSince {
			agentErrCh <- fmt.Errorf("expected follow-up since %q, got %q", wantSince, query.Since)
			return
		}
		if err := broker.StoreResult(dockerAgentQueryResult{
			QueryID:     query.QueryID,
			NodeID:      "main",
			PayloadJSON: mustMarshalDockerListResultForTest(dockerListResult{Content: "2026-04-18T10:00:01Z world\n"}),
		}); err != nil {
			agentErrCh <- fmt.Errorf("store second docker query result: %w", err)
			return
		}
		agentErrCh <- nil
	}()

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.GetContainerLogs(streamCtx, connect.NewRequest(&controllerv1.GetContainerLogsRequest{NodeId: "main", ContainerId: "ctr", Tail: "2"}))
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

	cancel()
	if err := <-agentErrCh; err != nil {
		t.Fatal(err)
	}
}

func waitForDockerQueryForTest(broker *dockerQueryBroker, nodeID string, timeout time.Duration) (dockerAgentQuery, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if query, ok := broker.Pull(nodeID); ok {
			return query, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return dockerAgentQuery{}, fmt.Errorf("timed out waiting for docker query on node %q", nodeID)
}

func mustMarshalDockerListResultForTest(result dockerListResult) string {
	payload, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return string(payload)
}
