package controller

import (
	"strings"
	"testing"
	"time"
)

func TestDockerQueryBrokerExpiresPendingAssignedAndResults(t *testing.T) {
	t.Parallel()

	broker := newDockerQueryBroker()
	expired := time.Now().UTC().Add(-time.Second)

	broker.Enqueue(dockerAgentQuery{QueryID: "pending", NodeID: "main", expiresAt: expired})
	if _, ok := broker.Pull("main"); ok {
		t.Fatal("expected expired pending query to be skipped")
	}

	broker.Enqueue(dockerAgentQuery{QueryID: "assigned", NodeID: "main"})
	if _, ok := broker.Pull("main"); !ok {
		t.Fatal("expected query to be assigned")
	}
	broker.mu.Lock()
	assigned := broker.assignedByID["assigned"]
	assigned.expiresAt = expired
	broker.assignedByID["assigned"] = assigned
	broker.mu.Unlock()
	if err := broker.StoreResult(dockerAgentQueryResult{QueryID: "assigned", NodeID: "main"}); err == nil || !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("expected expired assigned query to reject result, got %v", err)
	}

	broker.Enqueue(dockerAgentQuery{QueryID: "result", NodeID: "main"})
	if _, ok := broker.Pull("main"); !ok {
		t.Fatal("expected query to be assigned for result")
	}
	if err := broker.StoreResult(dockerAgentQueryResult{QueryID: "result", NodeID: "main", PayloadJSON: "{}"}); err != nil {
		t.Fatalf("store result: %v", err)
	}
	broker.mu.Lock()
	stored := broker.results["result"]
	stored.expiresAt = expired
	broker.results["result"] = stored
	broker.mu.Unlock()
	if _, ok := broker.PopResult("result"); ok {
		t.Fatal("expected expired result to be removed")
	}
}
