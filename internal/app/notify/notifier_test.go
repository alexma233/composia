package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

func TestNewNotifierWithNilOrDisabledConfig(t *testing.T) {
	t.Parallel()

	notifier, err := New(nil)
	if err != nil {
		t.Fatalf("New nil returned error: %v", err)
	}
	if notifier == nil || len(notifier.routes) != 0 {
		t.Fatalf("unexpected nil-config notifier: %+v", notifier)
	}
	disabled := false
	notifier, err = New(&config.ControllerNotificationsConfig{SMTP: &config.ControllerSMTPNotificationConfig{Enabled: &disabled}, Telegram: &config.ControllerTelegramNotificationConfig{Enabled: &disabled}})
	if err != nil {
		t.Fatalf("New disabled returned error: %v", err)
	}
	if len(notifier.routes) != 0 {
		t.Fatalf("disabled routes = %+v", notifier.routes)
	}
}

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

	if err := notifier.Send(context.Background(), Event{
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
	}); err != nil {
		t.Fatal(err)
	}

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

func TestNotifierSendReturnsDeliveryFailure(t *testing.T) {
	t.Parallel()
	sender := &recordingSender{err: errors.New("delivery failed")}
	notifier := &Notifier{routes: []route{{name: "test", sender: sender}}}
	err := notifier.Send(context.Background(), Event{Type: corenotify.EventTaskFailed, Task: &TaskEvent{TaskID: "task-1"}})
	if err == nil || !strings.Contains(err.Error(), "delivery failed") {
		t.Fatalf("expected delivery failure, got %v", err)
	}
}

type recordingSender struct {
	calls   int
	subject string
	body    string
	err     error
}

func (sender *recordingSender) Send(_ context.Context, subject, body string) error {
	sender.calls++
	sender.subject = subject
	sender.body = body
	return sender.err
}
