package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const taskLeaseRenewInterval = 20 * time.Second

type taskExecutionContextKey struct{}

type persistedTaskExecution struct {
	TaskID        string                  `json:"task_id"`
	ExecutionID   string                  `json:"execution_id"`
	Acknowledged  bool                    `json:"acknowledged"`
	Completion    *completion             `json:"completion,omitempty"`
	BackupResults []persistedBackupResult `json:"backup_results,omitempty"`
}

type persistedBackupResult struct {
	ServiceName  string      `json:"service_name"`
	DataName     string      `json:"data_name"`
	ArtifactRef  string      `json:"artifact_ref,omitempty"`
	Status       task.Status `json:"status"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   time.Time   `json:"finished_at"`
	ErrorSummary string      `json:"error_summary,omitempty"`
}

type completion struct {
	Status       task.Status `json:"status"`
	ErrorSummary string      `json:"error_summary,omitempty"`
	FinishedAt   time.Time   `json:"finished_at"`
}

type taskExecutionRuntime struct {
	mu    sync.Mutex
	path  string
	state persistedTaskExecution
}

func taskExecutionStatePath(stateDir string) string {
	return filepath.Join(stateDir, "task-execution.json")
}

func withTaskExecution(ctx context.Context, runtime *taskExecutionRuntime) context.Context {
	return context.WithValue(ctx, taskExecutionContextKey{}, runtime)
}

func taskExecutionFromContext(ctx context.Context) *taskExecutionRuntime {
	runtime, _ := ctx.Value(taskExecutionContextKey{}).(*taskExecutionRuntime)
	return runtime
}

func taskExecutionID(ctx context.Context) string {
	if runtime := taskExecutionFromContext(ctx); runtime != nil {
		return runtime.state.ExecutionID
	}
	return ""
}

func taskExecutionTaskID(ctx context.Context) string {
	if runtime := taskExecutionFromContext(ctx); runtime != nil {
		return runtime.state.TaskID
	}
	return ""
}

func persistTaskExecution(path string, state persistedTaskExecution) error {
	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode task execution state: %w", err)
	}
	tempPath := path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("write task execution state: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write task execution state: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync task execution state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close task execution state: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("activate task execution state: %w", err)
	}
	dir, err := os.Open(filepath.Dir(path)) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open task execution state directory: %w", err)
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync task execution state directory: %w", err)
	}
	return nil
}

func loadTaskExecution(path string) (persistedTaskExecution, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if errors.Is(err, os.ErrNotExist) {
		return persistedTaskExecution{}, nil
	}
	if err != nil {
		return persistedTaskExecution{}, fmt.Errorf("read task execution state: %w", err)
	}
	var state persistedTaskExecution
	if err := json.Unmarshal(content, &state); err != nil {
		return persistedTaskExecution{}, fmt.Errorf("decode task execution state: %w", err)
	}
	return state, nil
}

func acknowledgeTaskExecution(ctx context.Context, client agentv1connect.AgentTaskServiceClient, taskID, executionID string) error {
	callCtx, cancel := context.WithTimeout(ctx, taskReportTimeout)
	defer cancel()
	_, err := client.AcknowledgeTask(callCtx, connect.NewRequest(&agentv1.AcknowledgeTaskRequest{TaskId: taskID, ExecutionId: executionID}))
	if err != nil {
		return fmt.Errorf("acknowledge task execution: %w", err)
	}
	return nil
}

func isRejectedTaskExecution(err error) bool {
	code := connect.CodeOf(err)
	return code == connect.CodeFailedPrecondition || code == connect.CodeNotFound
}

func recoverTaskExecution(ctx context.Context, cfg *config.AgentConfig, taskClient agentv1connect.AgentTaskServiceClient, reportClient agentv1connect.AgentReportServiceClient) error {
	path := taskExecutionStatePath(cfg.StateDir)
	state, err := loadTaskExecution(path)
	if err != nil || state.TaskID == "" {
		return err
	}
	if state.Completion == nil && !state.Acknowledged {
		if err := acknowledgeTaskExecution(ctx, taskClient, state.TaskID, state.ExecutionID); err != nil {
			if isRejectedTaskExecution(err) {
				return removeTaskExecutionState(path)
			}
			return err
		} else {
			state.Acknowledged = true
			if err := persistTaskExecution(path, state); err != nil {
				return err
			}
		}
	}
	runtime := &taskExecutionRuntime{path: path, state: state}
	for len(runtime.state.BackupResults) > 0 {
		if err := sendPersistedBackupResult(ctx, reportClient, runtime.state.TaskID, runtime.state.ExecutionID, runtime.state.BackupResults[0]); err != nil {
			return err
		}
		if err := runtime.removeFirstBackupResult(); err != nil {
			return err
		}
	}
	state = runtime.state
	if state.Completion == nil {
		state.Completion = &completion{Status: task.StatusFailed, ErrorSummary: "agent restarted during an accepted task execution; outcome may be partial", FinishedAt: time.Now().UTC()}
		runtime.state = state
		if err := persistTaskExecution(path, state); err != nil {
			return err
		}
	}
	request := &agentv1.ReportTaskStateRequest{TaskId: state.TaskID, ExecutionId: state.ExecutionID, Status: protoAgentTaskStatus(state.Completion.Status), ErrorSummary: state.Completion.ErrorSummary, FinishedAt: timestamppb.New(state.Completion.FinishedAt)}
	callCtx, cancel := context.WithTimeout(ctx, taskReportTimeout)
	defer cancel()
	if _, err := reportClient.ReportTaskState(callCtx, connect.NewRequest(request)); err != nil {
		if isRejectedTaskExecution(err) {
			archivePath, archiveErr := archiveTaskExecutionState(path)
			if archiveErr != nil {
				return fmt.Errorf("archive conflicting task completion: %w", archiveErr)
			}
			log.Printf("task completion conflicted with controller state and was archived: task_id=%s archive=%s", state.TaskID, archivePath)
			return nil
		}
		return fmt.Errorf("recover task completion: %w", err)
	}
	return removeTaskExecutionState(path)
}

func archiveTaskExecutionState(path string) (string, error) {
	archivePath := filepath.Join(filepath.Dir(path), fmt.Sprintf("task-execution-conflict-%d.json", time.Now().UTC().UnixNano()))
	if err := os.Rename(path, archivePath); err != nil {
		return "", err
	}
	dir, err := os.Open(filepath.Dir(path)) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return "", err
	}
	return archivePath, nil
}

func removeTaskExecutionState(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func renewTaskLease(ctx context.Context, client agentv1connect.AgentTaskServiceClient, taskID, executionID string) {
	ticker := time.NewTicker(taskLeaseRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, taskReportTimeout)
			_, err := client.RenewTaskLease(callCtx, connect.NewRequest(&agentv1.RenewTaskLeaseRequest{TaskId: taskID, ExecutionId: executionID}))
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Printf("renew task lease failed: task_id=%s error=%v", taskID, err)
			}
		}
	}
}

func (runtime *taskExecutionRuntime) storeCompletion(value completion) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if len(runtime.state.BackupResults) > 0 {
		return errors.New("task completion cannot be persisted before backup results are acknowledged")
	}
	if runtime.state.Completion != nil {
		if runtime.state.Completion.Status != value.Status {
			return errors.New("task completion already persisted with a different status")
		}
		return nil
	}
	runtime.state.Completion = &value
	return persistTaskExecution(runtime.path, runtime.state)
}

func (runtime *taskExecutionRuntime) storeBackupResult(value persistedBackupResult) error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for _, existing := range runtime.state.BackupResults {
		if existing.DataName == value.DataName {
			if existing.Status != value.Status || existing.ArtifactRef != value.ArtifactRef {
				return errors.New("backup result already persisted with different content")
			}
			return nil
		}
	}
	runtime.state.BackupResults = append(runtime.state.BackupResults, value)
	return persistTaskExecution(runtime.path, runtime.state)
}

func (runtime *taskExecutionRuntime) removeFirstBackupResult() error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if len(runtime.state.BackupResults) == 0 {
		return nil
	}
	runtime.state.BackupResults = runtime.state.BackupResults[1:]
	return persistTaskExecution(runtime.path, runtime.state)
}

func (runtime *taskExecutionRuntime) clear() error {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	err := os.Remove(runtime.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
