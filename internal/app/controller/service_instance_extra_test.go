package controller

import (
	"testing"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestServiceInstanceMessages(t *testing.T) {
	t.Parallel()

	record := store.ServiceInstanceSnapshot{ServiceName: "app", NodeID: "main", RuntimeStatus: store.ServiceRuntimeRunning, UpdatedAt: "2026-05-31T04:05:00Z", IsDeclared: true, PendingDeployRevision: "rev-1"}
	summary := serviceInstanceSummaryMessage(record)
	if summary.GetServiceName() != "app" || summary.GetNodeId() != "main" || summary.GetPendingDeployRevision() != "rev-1" {
		t.Fatalf("summary = %+v", summary)
	}
	detail := serviceInstanceDetailMessage(record, []*controllerv1.ServiceContainerSummary{{Name: "app-1"}})
	if detail.GetServiceName() != "app" || len(detail.GetContainers()) != 1 || detail.GetContainers()[0].GetName() != "app-1" {
		t.Fatalf("detail = %+v", detail)
	}
}

func TestServiceContainerSummaryMessage(t *testing.T) {
	t.Parallel()

	if serviceContainerSummaryMessage(nil) != nil {
		t.Fatalf("nil container should return nil")
	}
	summary := serviceContainerSummaryMessage(&controllerv1.ContainerInfo{Id: "id", Name: "name", Image: "image", State: "running", Status: "Up", Created: "now", Labels: map[string]string{"com.docker.compose.project": "app", "com.docker.compose.service": "web"}})
	if summary.GetContainerId() != "id" || summary.GetComposeProject() != "app" || summary.GetComposeService() != "web" {
		t.Fatalf("summary = %+v", summary)
	}
}
