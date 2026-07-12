package controller

import (
	"testing"
	"time"

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
