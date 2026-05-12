package controller

import (
	"sync"
)

type taskQueueNotifier struct {
	mu          sync.Mutex
	subscribers map[chan struct{}]struct{}
}

func newTaskQueueNotifier() *taskQueueNotifier {
	return &taskQueueNotifier{subscribers: make(map[chan struct{}]struct{})}
}

func (notifier *taskQueueNotifier) Subscribe() chan struct{} {
	if notifier == nil {
		return nil
	}
	ch := make(chan struct{}, 1)
	notifier.mu.Lock()
	notifier.subscribers[ch] = struct{}{}
	notifier.mu.Unlock()
	return ch
}

func (notifier *taskQueueNotifier) Unsubscribe(ch chan struct{}) {
	if notifier == nil || ch == nil {
		return
	}
	notifier.mu.Lock()
	if _, ok := notifier.subscribers[ch]; ok {
		delete(notifier.subscribers, ch)
		close(ch)
	}
	notifier.mu.Unlock()
}

func (notifier *taskQueueNotifier) Notify() {
	if notifier == nil {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	for ch := range notifier.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

type taskResultNotifier struct {
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
}

func newTaskResultNotifier() *taskResultNotifier {
	return &taskResultNotifier{subscribers: make(map[string]map[chan struct{}]struct{})}
}

func (notifier *taskResultNotifier) Subscribe(taskID string) chan struct{} {
	if notifier == nil || taskID == "" {
		return nil
	}
	ch := make(chan struct{}, 1)
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if notifier.subscribers[taskID] == nil {
		notifier.subscribers[taskID] = make(map[chan struct{}]struct{})
	}
	notifier.subscribers[taskID][ch] = struct{}{}
	return ch
}

func (notifier *taskResultNotifier) Unsubscribe(taskID string, ch chan struct{}) {
	if notifier == nil || taskID == "" || ch == nil {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	subscribers := notifier.subscribers[taskID]
	if subscribers == nil {
		return
	}
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	if len(subscribers) == 0 {
		delete(notifier.subscribers, taskID)
	}
}

func (notifier *taskResultNotifier) Notify(taskID string) {
	if notifier == nil || taskID == "" {
		return
	}
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	for ch := range notifier.subscribers[taskID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
