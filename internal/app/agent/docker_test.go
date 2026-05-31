package agent

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestPaginateDockerList(t *testing.T) {
	t.Parallel()

	items, total := paginateDockerList([]int{1, 2, 3, 4, 5}, 2, 2)
	if total != 5 || len(items) != 2 || items[0] != 3 || items[1] != 4 {
		t.Fatalf("page 2 = %+v total=%d", items, total)
	}
	items, total = paginateDockerList([]int{1, 2, 3}, 0, 2)
	if total != 3 || len(items) != 2 || items[0] != 1 || items[1] != 2 {
		t.Fatalf("page 0 normalized = %+v total=%d", items, total)
	}
	items, total = paginateDockerList([]int{1, 2, 3}, 3, 2)
	if total != 3 || len(items) != 0 {
		t.Fatalf("out of range page = %+v total=%d", items, total)
	}
}

func TestDockerSearchAndStringHelpers(t *testing.T) {
	t.Parallel()

	if !dockerSearchMatches("WEB", "api", "web-1") {
		t.Fatalf("expected case-insensitive search match")
	}
	if dockerSearchMatches("db", "api", "web-1") {
		t.Fatalf("unexpected search match")
	}
	if got := firstNonEmpty("", "  ", "value"); got != "value" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := joinStrings([]string{"a", "b"}); got != "a b" {
		t.Fatalf("joinStrings = %q", got)
	}
}

func TestDockerCompareHelpers(t *testing.T) {
	t.Parallel()

	if boolCompare(true, false) <= 0 || boolCompare(false, true) >= 0 || boolCompare(true, true) != 0 {
		t.Fatalf("unexpected boolCompare results")
	}
	if int64Compare(1, 2) != -1 || int64Compare(2, 1) != 1 || int64Compare(2, 2) != 0 {
		t.Fatalf("unexpected int64Compare results")
	}
	if uint32Compare(1, 2) != -1 || uint32Compare(2, 1) != 1 || uint32Compare(2, 2) != 0 {
		t.Fatalf("unexpected uint32Compare results")
	}
	if stringCompare("Alpha", "bravo") >= 0 {
		t.Fatalf("expected case-insensitive string comparison")
	}
	if !dockerSortResult(1, true) || dockerSortResult(1, false) || dockerSortResult(0, true) {
		t.Fatalf("unexpected dockerSortResult")
	}
}

func TestDockerTaskActionResourceCoversAllTaskTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		taskType agentv1.AgentTaskType
		action   string
		resource string
	}{
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_START, action: dockerActionsStart, resource: dockerResourceContainer},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_STOP, action: dockerActionsStop, resource: dockerResourceContainer},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_RESTART, action: dockerActionsRestart, resource: dockerResourceContainer},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_CONTAINER, action: dockerActionsRemove, resource: dockerResourceContainer},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_NETWORK, action: dockerActionsRemove, resource: "network"},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_VOLUME, action: dockerActionsRemove, resource: "volume"},
		{taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_IMAGE, action: dockerActionsRemove, resource: "image"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.taskType.String(), func(t *testing.T) {
			t.Parallel()
			action, resource, ok := dockerTaskActionResource(tt.taskType)
			if !ok || action != tt.action || resource != tt.resource {
				t.Fatalf("dockerTaskActionResource = %q/%q/%v, want %q/%q/true", action, resource, ok, tt.action, tt.resource)
			}
		})
	}
	if _, _, ok := dockerTaskActionResource(agentv1.AgentTaskType_AGENT_TASK_TYPE_DEPLOY); ok {
		t.Fatalf("expected unsupported task type")
	}
}

func TestDockerTaskStepName(t *testing.T) {
	t.Parallel()

	if dockerTaskStepName(dockerActionsStart) != task.StepDockerStart {
		t.Fatalf("start step mismatch")
	}
	if dockerTaskStepName(dockerActionsStop) != task.StepDockerStop {
		t.Fatalf("stop step mismatch")
	}
	if dockerTaskStepName(dockerActionsRestart) != task.StepDockerRestart {
		t.Fatalf("restart step mismatch")
	}
	if dockerTaskStepName(dockerActionsRemove) != task.StepDockerRemove {
		t.Fatalf("remove step mismatch")
	}
}

func TestParseDockerTaskParamsRejectsMissingID(t *testing.T) {
	t.Parallel()

	_, err := parseDockerTaskParams(`{"id":""}`, agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_START)
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestDockerQueryErrorCode(t *testing.T) {
	t.Parallel()

	tests := map[connect.Code]string{
		connect.CodeInvalidArgument:    "invalid_argument",
		connect.CodeNotFound:           "not_found",
		connect.CodeFailedPrecondition: "failed_precondition",
		connect.CodePermissionDenied:   "permission_denied",
		connect.CodeDeadlineExceeded:   "deadline_exceeded",
		connect.CodeUnavailable:        "unavailable",
		connect.CodeInternal:           "internal",
	}
	for code, want := range tests {
		code, want := code, want
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			if got := dockerQueryErrorCode(connect.NewError(code, nil)); got != want {
				t.Fatalf("dockerQueryErrorCode = %q, want %q", got, want)
			}
		})
	}
	if got := dockerQueryErrorCode(nil); got != "internal" {
		t.Fatalf("nil error code = %q", got)
	}
}
