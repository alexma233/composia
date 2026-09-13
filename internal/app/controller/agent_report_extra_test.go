package controller

import (
	"testing"
	"time"

	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBackupResultTimesPreserveAgentTimes(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 7, 12, 10, 0, 0, 123, time.UTC)
	finished := started.Add(2*time.Minute + time.Second)
	startedText, finishedText, err := backupResultTimes(timestamppb.New(started), timestamppb.New(finished))
	if err != nil {
		t.Fatal(err)
	}
	if startedText != started.Format(time.RFC3339Nano) || finishedText != finished.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected backup times: %q %q", startedText, finishedText)
	}
	if _, _, err := backupResultTimes(timestamppb.New(finished), timestamppb.New(started)); err == nil {
		t.Fatal("expected reversed backup times to fail")
	}
}

func TestServiceDigestUpdateAvailableAggregatesNodeStates(t *testing.T) {
	t.Parallel()
	checkedAt := time.Date(2026, 9, 13, 6, 0, 0, 0, time.UTC)
	check := store.ServiceImageUpdateCheck{
		ImageRef:        "registry:5000/example/app",
		CurrentTag:      "latest",
		CandidateDigest: "sha256:new",
	}
	states := []store.ServiceImageState{
		{NodeID: "main", ImageRef: "registry:5000/example/app", LocalDigest: "sha256:new", CheckStatus: store.ImageCheckStatusOK, CheckedAt: checkedAt},
		{NodeID: "edge", ImageRef: "registry:5000/example/app:latest", LocalDigest: "sha256:old", CheckStatus: store.ImageCheckStatusOK, CheckedAt: checkedAt},
		{NodeID: "other", ImageRef: "registry:5000/example/app:stable", LocalDigest: "sha256:old", CheckStatus: store.ImageCheckStatusOK, CheckedAt: checkedAt},
	}
	checkedBefore := checkedAt.Add(time.Second)
	_, available, errorSummary := serviceDigestUpdateState(check, states, []string{"main", "edge"}, checkedAt, checkedBefore)
	if !available || errorSummary != "" {
		t.Fatal("expected one stale target node to make the service image update available")
	}
	states[1].LocalDigest = "sha256:new"
	currentDigest, available, errorSummary := serviceDigestUpdateState(check, states, []string{"main", "edge"}, checkedAt, checkedBefore)
	if available || errorSummary != "" || currentDigest != "sha256:new" {
		t.Fatal("expected unrelated tags to be ignored when all matching nodes are current")
	}
	states[1].CheckStatus = store.ImageCheckStatusError
	if _, available, errorSummary := serviceDigestUpdateState(check, states, []string{"main", "edge"}, checkedAt, checkedBefore); available || errorSummary == "" {
		t.Fatal("expected a failed local observation to block the aggregate result")
	}
	states[1].CheckStatus = store.ImageCheckStatusOK
	states[1].CheckedAt = checkedAt.Add(-time.Second)
	if _, available, errorSummary := serviceDigestUpdateState(check, states, []string{"main", "edge"}, checkedAt, checkedBefore); available || errorSummary == "" {
		t.Fatal("expected a stale local observation to be ignored")
	}
}
