package cli

import (
	"bytes"
	"strings"
	"testing"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func TestServiceDetailIncludesRecentConsistencyWithoutContainers(t *testing.T) {
	t.Parallel()
	service := &controllerv1.GetServiceResponse{Name: "demo", Instances: []*controllerv1.ServiceInstanceDetail{
		{ServiceName: "demo", NodeId: "main", Consistency: &agentv1.ServiceConsistencyCheck{
			Files:     &agentv1.ServiceConsistencyOutcome{Status: "consistent"},
			Compose:   &agentv1.ServiceConsistencyOutcome{Status: "drifted", Reasons: []string{"config differs"}},
			CheckedAt: "2026-09-22T12:00:00Z", RepoRevision: "revision", TaskId: "task-id",
		}},
		{ServiceName: "demo", NodeId: "unchecked"},
	}}
	var output bytes.Buffer
	application := &app{out: &output}
	if err := application.printServiceDetail(service); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RECENT FILES", "RECENT COMPOSE", "CHECKED AT", "consistent", "drifted", "config differs", "revision", "task-id", "2026-09-22T12:00:00Z", "unknown"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q in %s", want, output.String())
		}
	}
	output.Reset()
	application.cfg.output = outputModeJSON
	if err := application.printMessage(service); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"consistency"`, `"files"`, `"compose"`, `"checked_at"`, `"repo_revision"`, `"task_id"`, `"config differs"`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing JSON field %q in %s", want, output.String())
		}
	}
}
