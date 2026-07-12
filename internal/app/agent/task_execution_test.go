package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestTaskExecutionStatePersistsCompletionAndBackupOrdering(t *testing.T) {
	t.Parallel()
	path := taskExecutionStatePath(t.TempDir())
	state := persistedTaskExecution{TaskID: "task-1", ExecutionID: "execution-1", Acknowledged: true}
	if err := persistTaskExecution(path, state); err != nil {
		t.Fatal(err)
	}
	runtime := &taskExecutionRuntime{path: path, state: state}
	result := persistedBackupResult{DataName: "db", Status: task.StatusSucceeded, ArtifactRef: "snapshot", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC()}
	if err := runtime.storeBackupResult(result); err != nil {
		t.Fatal(err)
	}
	if err := runtime.storeCompletion(completion{Status: task.StatusSucceeded, FinishedAt: time.Now().UTC()}); err == nil {
		t.Fatal("expected completion to wait for backup acknowledgement")
	}
	if err := runtime.removeFirstBackupResult(); err != nil {
		t.Fatal(err)
	}
	if err := runtime.storeCompletion(completion{Status: task.StatusSucceeded, FinishedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadTaskExecution(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Completion == nil || loaded.Completion.Status != task.StatusSucceeded {
		t.Fatalf("unexpected persisted execution: %+v", loaded)
	}
	if err := runtime.clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected execution state removal, got %v", err)
	}
}

func TestRecoverTaskExecutionArchivesConflictingCompletion(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	path := taskExecutionStatePath(stateDir)
	state := persistedTaskExecution{TaskID: "task-1", ExecutionID: "execution-1", Acknowledged: true, Completion: &completion{Status: task.StatusSucceeded, FinishedAt: time.Now().UTC()}}
	if err := persistTaskExecution(path, state); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	servicePath, handler := agentv1connect.NewAgentReportServiceHandler(&conflictingCompletionServer{})
	mux.Handle(servicePath, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	reportClient := agentv1connect.NewAgentReportServiceClient(server.Client(), server.URL)
	err := recoverTaskExecution(context.Background(), &config.AgentConfig{StateDir: stateDir}, nil, reportClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected active execution state to be cleared, got %v", err)
	}
	archives, err := filepath.Glob(filepath.Join(stateDir, "task-execution-conflict-*.json"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("expected one archived completion, archives=%v err=%v", archives, err)
	}
	archived, err := loadTaskExecution(archives[0])
	if err != nil || archived.Completion == nil || archived.Completion.Status != task.StatusSucceeded {
		t.Fatalf("unexpected archived completion: state=%+v err=%v", archived, err)
	}
}

func TestRecoverTaskExecutionDiscardsExpiredUnacknowledgedOffer(t *testing.T) {
	t.Parallel()
	stateDir := t.TempDir()
	path := taskExecutionStatePath(stateDir)
	if err := persistTaskExecution(path, persistedTaskExecution{TaskID: "task-stale", ExecutionID: "execution-stale"}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	servicePath, handler := agentv1connect.NewAgentTaskServiceHandler(&rejectedAcknowledgementServer{})
	mux.Handle(servicePath, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	taskClient := agentv1connect.NewAgentTaskServiceClient(server.Client(), server.URL)
	if err := recoverTaskExecution(context.Background(), &config.AgentConfig{StateDir: stateDir}, taskClient, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale unacknowledged execution to be discarded, got %v", err)
	}
}

type conflictingCompletionServer struct {
	agentv1connect.UnimplementedAgentReportServiceHandler
}

type rejectedAcknowledgementServer struct {
	agentv1connect.UnimplementedAgentTaskServiceHandler
}

func (*rejectedAcknowledgementServer) AcknowledgeTask(context.Context, *connect.Request[agentv1.AcknowledgeTaskRequest]) (*connect.Response[agentv1.AcknowledgeTaskResponse], error) {
	return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("offer expired"))
}

func (*conflictingCompletionServer) ReportTaskState(context.Context, *connect.Request[agentv1.ReportTaskStateRequest]) (*connect.Response[agentv1.ReportTaskStateResponse], error) {
	return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("conflict"))
}
