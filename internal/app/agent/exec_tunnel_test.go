package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
)

type recordingExecTunnelStream struct {
	mu            sync.Mutex
	messages      []*agentv1.OpenExecTunnelRequest
	inSend        int
	maxConcurrent int
}

func (stream *recordingExecTunnelStream) Send(message *agentv1.OpenExecTunnelRequest) error {
	stream.mu.Lock()
	stream.inSend++
	if stream.inSend > stream.maxConcurrent {
		stream.maxConcurrent = stream.inSend
	}
	stream.mu.Unlock()

	time.Sleep(time.Millisecond)

	stream.mu.Lock()
	stream.messages = append(stream.messages, message)
	stream.inSend--
	stream.mu.Unlock()
	return nil
}

func TestRunningExecSessionsDeleteIsInstanceSafe(t *testing.T) {
	t.Parallel()

	sessions := &runningExecSessions{sessions: make(map[string]*runningExecSession)}
	oldSession := &runningExecSession{id: "session"}
	newSession := &runningExecSession{id: "session"}
	sessions.sessions["session"] = newSession

	sessions.deleteIfCurrent(oldSession)
	if got := sessions.get("session"); got != newSession {
		t.Fatalf("stale cleanup removed current session: %+v", got)
	}
	sessions.deleteIfCurrent(newSession)
	if got := sessions.get("session"); got != nil {
		t.Fatalf("current cleanup left session: %+v", got)
	}
}

func TestExecTunnelSenderSerializesConcurrentSends(t *testing.T) {
	t.Parallel()

	stream := &recordingExecTunnelStream{}
	sender := newExecTunnelSender(context.Background(), stream)
	var wg sync.WaitGroup
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sender.send(&agentv1.OpenExecTunnelRequest{Kind: execKindOutput}); err != nil {
				t.Errorf("send exec tunnel message: %v", err)
			}
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(time.Second)
	for {
		stream.mu.Lock()
		messageCount := len(stream.messages)
		stream.mu.Unlock()
		if messageCount == 25 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := sender.close(); err != nil {
		t.Fatalf("close sender: %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.maxConcurrent != 1 {
		t.Fatalf("expected serialized sends, max concurrency was %d", stream.maxConcurrent)
	}
	if len(stream.messages) != 25 {
		t.Fatalf("expected 25 messages, got %d", len(stream.messages))
	}
}
