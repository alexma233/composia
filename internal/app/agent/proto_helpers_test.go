package agent

import (
	"testing"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
)

func TestApplyDockerQueryResultUsesQueryTypeForLists(t *testing.T) {
	tests := []struct {
		name  string
		query *agentv1.DockerQueryTask
		check func(t *testing.T, request *agentv1.ReportDockerQueryResultRequest)
	}{
		{
			name:  "containers",
			query: &agentv1.DockerQueryTask{Query: &agentv1.DockerQueryTask_ListContainers{ListContainers: &agentv1.ListContainersRequest{}}},
			check: func(t *testing.T, request *agentv1.ReportDockerQueryResultRequest) {
				if _, ok := request.Result.(*agentv1.ReportDockerQueryResultRequest_ListContainers); !ok {
					t.Fatalf("expected list containers result, got %T", request.Result)
				}
			},
		},
		{
			name:  "networks",
			query: &agentv1.DockerQueryTask{Query: &agentv1.DockerQueryTask_ListNetworks{ListNetworks: &agentv1.ListNetworksRequest{}}},
			check: func(t *testing.T, request *agentv1.ReportDockerQueryResultRequest) {
				if _, ok := request.Result.(*agentv1.ReportDockerQueryResultRequest_ListNetworks); !ok {
					t.Fatalf("expected list networks result, got %T", request.Result)
				}
			},
		},
		{
			name:  "volumes",
			query: &agentv1.DockerQueryTask{Query: &agentv1.DockerQueryTask_ListVolumes{ListVolumes: &agentv1.ListVolumesRequest{}}},
			check: func(t *testing.T, request *agentv1.ReportDockerQueryResultRequest) {
				if _, ok := request.Result.(*agentv1.ReportDockerQueryResultRequest_ListVolumes); !ok {
					t.Fatalf("expected list volumes result, got %T", request.Result)
				}
			},
		},
		{
			name:  "images",
			query: &agentv1.DockerQueryTask{Query: &agentv1.DockerQueryTask_ListImages{ListImages: &agentv1.ListImagesRequest{}}},
			check: func(t *testing.T, request *agentv1.ReportDockerQueryResultRequest) {
				if _, ok := request.Result.(*agentv1.ReportDockerQueryResultRequest_ListImages); !ok {
					t.Fatalf("expected list images result, got %T", request.Result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, totalCount := range []uint32{0, 1} {
				request := &agentv1.ReportDockerQueryResultRequest{}
				applyDockerQueryResult(request, tt.query, dockerTaskResult{TotalCount: totalCount})
				tt.check(t, request)
			}
		})
	}
}

func TestParseDockerTaskParamsInfersRemoveResourceFromTaskType(t *testing.T) {
	tests := []struct {
		name     string
		taskType agentv1.AgentTaskType
		resource string
	}{
		{name: "container", taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_CONTAINER, resource: dockerResourceContainer},
		{name: "network", taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_NETWORK, resource: "network"},
		{name: "volume", taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_VOLUME, resource: "volume"},
		{name: "image", taskType: agentv1.AgentTaskType_AGENT_TASK_TYPE_DOCKER_REMOVE_IMAGE, resource: "image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := parseDockerTaskParams(`{"action":"`+dockerActionsRemove+`","resource":"wrong","id":"resource-id"}`, tt.taskType)
			if err != nil {
				t.Fatalf("parse docker task params: %v", err)
			}
			if params.Action != dockerActionsRemove || params.Resource != tt.resource || params.ID != "resource-id" {
				t.Fatalf("unexpected params: %+v", params)
			}
		})
	}
}
