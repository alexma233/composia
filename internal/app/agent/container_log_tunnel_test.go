package agent

import "testing"

func TestRunningContainerLogSessionsDeleteIsInstanceSafe(t *testing.T) {
	t.Parallel()

	sessions := &runningContainerLogSessions{sessions: make(map[string]*runningContainerLogSession)}
	oldSession := &runningContainerLogSession{id: "session"}
	newSession := &runningContainerLogSession{id: "session"}
	sessions.sessions["session"] = newSession

	sessions.deleteIfCurrent(oldSession)
	if got := sessions.sessions["session"]; got != newSession {
		t.Fatalf("stale cleanup removed current session: %+v", got)
	}
	sessions.deleteIfCurrent(newSession)
	if got := sessions.sessions["session"]; got != nil {
		t.Fatalf("current cleanup left session: %+v", got)
	}
}
