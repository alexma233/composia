package notify

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	corenotify "forgejo.alexma.top/alexma233/composia/internal/core/notify"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
)

const dispatchTimeout = 10 * time.Second

type sender interface {
	Send(context.Context, string, string) error
}

type route struct {
	name    string
	events  map[corenotify.EventType]struct{}
	sources map[task.Source]struct{}
	sender  sender
}

type Notifier struct {
	routes []route
}

func New(cfg *config.ControllerNotificationsConfig) (*Notifier, error) {
	notifier := &Notifier{}
	if cfg == nil {
		return notifier, nil
	}
	if cfg.SMTP != nil && cfg.SMTP.IsEnabled() {
		sender, err := newSMTPSender(cfg.SMTP)
		if err != nil {
			return nil, err
		}
		notifier.routes = append(notifier.routes, route{
			name:    "smtp",
			events:  buildEventFilter(cfg.SMTP.On),
			sources: buildSourceFilter(cfg.SMTP.TaskSources),
			sender:  sender,
		})
	}
	if cfg.Telegram != nil && cfg.Telegram.IsEnabled() {
		sender, err := newTelegramSender(cfg.Telegram)
		if err != nil {
			return nil, err
		}
		notifier.routes = append(notifier.routes, route{
			name:    "telegram",
			events:  buildEventFilter(cfg.Telegram.On),
			sources: buildSourceFilter(cfg.Telegram.TaskSources),
			sender:  sender,
		})
	}
	return notifier, nil
}

func (notifier *Notifier) Dispatch(event Event) {
	go func() {
		if err := notifier.Send(context.Background(), event); err != nil {
			log.Printf("notification failed for %s: %v", event.Type, err)
		}
	}()
}

func (notifier *Notifier) Send(ctx context.Context, event Event) error {
	if notifier == nil || len(notifier.routes) == 0 {
		return nil
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	subject, body, err := renderEvent(event)
	if err != nil {
		return fmt.Errorf("render notification %s: %w", event.Type, err)
	}
	ctx, cancel := context.WithTimeout(ctx, dispatchTimeout)
	defer cancel()
	var sendErrors []error
	for _, route := range notifier.routes {
		if !route.matches(event) {
			continue
		}
		if err := route.sender.Send(ctx, subject, body); err != nil {
			sendErrors = append(sendErrors, fmt.Errorf("send notification via %s: %w", route.name, err))
		}
	}
	return errors.Join(sendErrors...)
}

func (route route) matches(event Event) bool {
	if len(route.events) > 0 {
		if _, ok := route.events[event.Type]; !ok {
			return false
		}
	}
	if len(route.sources) == 0 || event.Source == "" {
		return true
	}
	_, ok := route.sources[event.Source]
	return ok
}

func buildEventFilter(values []string) map[corenotify.EventType]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[corenotify.EventType]struct{}, len(values))
	for _, value := range values {
		result[corenotify.EventType(strings.TrimSpace(strings.ToLower(value)))] = struct{}{}
	}
	return result
}

func buildSourceFilter(values []string) map[task.Source]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[task.Source]struct{}, len(values))
	for _, value := range values {
		result[task.Source(strings.TrimSpace(strings.ToLower(value)))] = struct{}{}
	}
	return result
}

func taskLabel(serviceName, nodeID string) string {
	serviceName = strings.TrimSpace(serviceName)
	nodeID = strings.TrimSpace(nodeID)
	switch {
	case serviceName != "" && nodeID != "":
		return fmt.Sprintf("%s@%s", serviceName, nodeID)
	case serviceName != "":
		return serviceName
	case nodeID != "":
		return nodeID
	default:
		return "unknown"
	}
}
