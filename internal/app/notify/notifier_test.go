package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestRouteMatchesEventAndSourceFilters(t *testing.T) {
	t.Parallel()

	route := route{
		events:  buildEventFilter([]string{" task_failed "}),
		sources: buildSourceFilter([]string{" cli "}),
	}
	if !route.matches(Event{Type: corenotify.EventTaskFailed, Source: task.SourceCLI}) {
		t.Fatalf("expected route to match configured event and source")
	}
	if route.matches(Event{Type: corenotify.EventTaskCompleted, Source: task.SourceCLI}) {
		t.Fatalf("expected route to reject unconfigured event")
	}
	if route.matches(Event{Type: corenotify.EventTaskFailed, Source: task.SourceWeb}) {
		t.Fatalf("expected route to reject unconfigured source")
	}
	if !route.matches(Event{Type: corenotify.EventTaskFailed}) {
		t.Fatalf("empty event source should not block delivery")
	}
}

func TestTaskLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		service string
		node    string
		want    string
	}{
		{service: " web ", node: " main ", want: "web@main"},
		{service: "web", want: "web"},
		{node: "main", want: "main"},
		{want: "unknown"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := taskLabel(tt.service, tt.node); got != tt.want {
				t.Fatalf("taskLabel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotifierDispatchSendsMatchingRoutes(t *testing.T) {
	t.Parallel()

	matchingSender := &recordingSender{}
	skippedSender := &recordingSender{}
	notifier := &Notifier{routes: []route{
		{
			name:    "matching",
			events:  buildEventFilter([]string{string(corenotify.EventTaskFailed)}),
			sources: buildSourceFilter([]string{string(task.SourceCLI)}),
			sender:  matchingSender,
		},
		{
			name:    "skipped",
			events:  buildEventFilter([]string{string(corenotify.EventTaskCompleted)}),
			sources: buildSourceFilter([]string{string(task.SourceCLI)}),
			sender:  skippedSender,
		},
	}}

	notifier.dispatch(Event{
		Type:       corenotify.EventTaskFailed,
		OccurredAt: time.Date(2026, 5, 31, 4, 5, 0, 0, time.UTC),
		Source:     task.SourceCLI,
		Task: &TaskEvent{
			TaskID:      "task-1",
			TaskType:    task.TypeDeploy,
			Status:      task.StatusFailed,
			ServiceName: "web",
			NodeID:      "main",
		},
	})

	if matchingSender.calls != 1 {
		t.Fatalf("matching sender calls = %d", matchingSender.calls)
	}
	if skippedSender.calls != 0 {
		t.Fatalf("skipped sender calls = %d", skippedSender.calls)
	}
	if !strings.Contains(matchingSender.subject, "task_failed web@main") || !strings.Contains(matchingSender.body, "Task ID: task-1") {
		t.Fatalf("unexpected notification subject/body: %q\n%s", matchingSender.subject, matchingSender.body)
	}
}

type recordingSender struct {
	calls   int
	subject string
	body    string
}

func (sender *recordingSender) Send(_ context.Context, subject, body string) error {
	sender.calls++
	sender.subject = subject
	sender.body = body
	return nil
}
