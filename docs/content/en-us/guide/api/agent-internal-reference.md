# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [proto/composia/agent/v1/agent.proto](#proto_composia_agent_v1_agent-proto)
    - [AgentTask](#composia-agent-v1-AgentTask)
    - [ContainerInfo](#composia-agent-v1-ContainerInfo)
    - [ContainerInfo.LabelsEntry](#composia-agent-v1-ContainerInfo-LabelsEntry)
    - [DockerQueryTask](#composia-agent-v1-DockerQueryTask)
    - [DockerStats](#composia-agent-v1-DockerStats)
    - [GetContainerLogsRequest](#composia-agent-v1-GetContainerLogsRequest)
    - [GetContainerLogsResponse](#composia-agent-v1-GetContainerLogsResponse)
    - [GetServiceBundleRequest](#composia-agent-v1-GetServiceBundleRequest)
    - [GetServiceBundleResponse](#composia-agent-v1-GetServiceBundleResponse)
    - [HeartbeatRequest](#composia-agent-v1-HeartbeatRequest)
    - [HeartbeatResponse](#composia-agent-v1-HeartbeatResponse)
    - [ImageInfo](#composia-agent-v1-ImageInfo)
    - [InspectContainerRequest](#composia-agent-v1-InspectContainerRequest)
    - [InspectContainerResponse](#composia-agent-v1-InspectContainerResponse)
    - [InspectImageRequest](#composia-agent-v1-InspectImageRequest)
    - [InspectImageResponse](#composia-agent-v1-InspectImageResponse)
    - [InspectNetworkRequest](#composia-agent-v1-InspectNetworkRequest)
    - [InspectNetworkResponse](#composia-agent-v1-InspectNetworkResponse)
    - [InspectVolumeRequest](#composia-agent-v1-InspectVolumeRequest)
    - [InspectVolumeResponse](#composia-agent-v1-InspectVolumeResponse)
    - [ListContainersRequest](#composia-agent-v1-ListContainersRequest)
    - [ListContainersResponse](#composia-agent-v1-ListContainersResponse)
    - [ListImagesRequest](#composia-agent-v1-ListImagesRequest)
    - [ListImagesResponse](#composia-agent-v1-ListImagesResponse)
    - [ListNetworksRequest](#composia-agent-v1-ListNetworksRequest)
    - [ListNetworksResponse](#composia-agent-v1-ListNetworksResponse)
    - [ListVolumesRequest](#composia-agent-v1-ListVolumesRequest)
    - [ListVolumesResponse](#composia-agent-v1-ListVolumesResponse)
    - [NetworkInfo](#composia-agent-v1-NetworkInfo)
    - [NetworkInfo.LabelsEntry](#composia-agent-v1-NetworkInfo-LabelsEntry)
    - [NodeRuntimeSummary](#composia-agent-v1-NodeRuntimeSummary)
    - [OpenContainerLogTunnelRequest](#composia-agent-v1-OpenContainerLogTunnelRequest)
    - [OpenContainerLogTunnelResponse](#composia-agent-v1-OpenContainerLogTunnelResponse)
    - [OpenExecTunnelRequest](#composia-agent-v1-OpenExecTunnelRequest)
    - [OpenExecTunnelResponse](#composia-agent-v1-OpenExecTunnelResponse)
    - [PullNextDockerQueryRequest](#composia-agent-v1-PullNextDockerQueryRequest)
    - [PullNextDockerQueryResponse](#composia-agent-v1-PullNextDockerQueryResponse)
    - [PullNextTaskRequest](#composia-agent-v1-PullNextTaskRequest)
    - [PullNextTaskResponse](#composia-agent-v1-PullNextTaskResponse)
    - [RemoveContainerRequest](#composia-agent-v1-RemoveContainerRequest)
    - [RemoveContainerResponse](#composia-agent-v1-RemoveContainerResponse)
    - [RemoveImageRequest](#composia-agent-v1-RemoveImageRequest)
    - [RemoveImageResponse](#composia-agent-v1-RemoveImageResponse)
    - [RemoveNetworkRequest](#composia-agent-v1-RemoveNetworkRequest)
    - [RemoveNetworkResponse](#composia-agent-v1-RemoveNetworkResponse)
    - [RemoveVolumeRequest](#composia-agent-v1-RemoveVolumeRequest)
    - [RemoveVolumeResponse](#composia-agent-v1-RemoveVolumeResponse)
    - [ReportBackupResultRequest](#composia-agent-v1-ReportBackupResultRequest)
    - [ReportBackupResultResponse](#composia-agent-v1-ReportBackupResultResponse)
    - [ReportDockerQueryResultRequest](#composia-agent-v1-ReportDockerQueryResultRequest)
    - [ReportDockerQueryResultResponse](#composia-agent-v1-ReportDockerQueryResultResponse)
    - [ReportDockerStatsRequest](#composia-agent-v1-ReportDockerStatsRequest)
    - [ReportDockerStatsResponse](#composia-agent-v1-ReportDockerStatsResponse)
    - [ReportServiceInstanceStatusRequest](#composia-agent-v1-ReportServiceInstanceStatusRequest)
    - [ReportServiceInstanceStatusResponse](#composia-agent-v1-ReportServiceInstanceStatusResponse)
    - [ReportTaskStateRequest](#composia-agent-v1-ReportTaskStateRequest)
    - [ReportTaskStateResponse](#composia-agent-v1-ReportTaskStateResponse)
    - [ReportTaskStepStateRequest](#composia-agent-v1-ReportTaskStepStateRequest)
    - [ReportTaskStepStateResponse](#composia-agent-v1-ReportTaskStepStateResponse)
    - [RunContainerActionRequest](#composia-agent-v1-RunContainerActionRequest)
    - [RunContainerActionResponse](#composia-agent-v1-RunContainerActionResponse)
    - [UploadTaskLogsRequest](#composia-agent-v1-UploadTaskLogsRequest)
    - [UploadTaskLogsResponse](#composia-agent-v1-UploadTaskLogsResponse)
    - [VolumeInfo](#composia-agent-v1-VolumeInfo)
    - [VolumeInfo.LabelsEntry](#composia-agent-v1-VolumeInfo-LabelsEntry)
  
    - [ContainerAction](#composia-agent-v1-ContainerAction)
  
    - [AgentReportService](#composia-agent-v1-AgentReportService)
    - [AgentTaskService](#composia-agent-v1-AgentTaskService)
    - [BundleService](#composia-agent-v1-BundleService)
    - [DockerService](#composia-agent-v1-DockerService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="proto_composia_agent_v1_agent-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/agent/v1/agent.proto



<a name="composia-agent-v1-AgentTask"></a>

### AgentTask
AgentTask describes one executable task assigned to an agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID assigned to the agent. |
| type | [string](#string) |  | type is the task type string assigned by the controller. |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id is the target node ID for the task. |
| repo_revision | [string](#string) |  | repo_revision is the desired repo revision for this task. |
| service_dir | [string](#string) |  | service_dir is the task service directory path within the bundle. |
| data_names | [string](#string) | repeated | data_names lists selected data entries for backup-like tasks. |
| params_json | [string](#string) |  | params_json stores task-type-specific JSON parameters. |






<a name="composia-agent-v1-ContainerInfo"></a>

### ContainerInfo
ContainerInfo describes one container for list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | name is the runtime container name. |
| image | [string](#string) |  |  |
| state | [string](#string) |  | state is the low-level Docker state value. |
| status | [string](#string) |  | status is the Docker status string intended for display. |
| created | [string](#string) |  | created is the container creation timestamp string. |
| labels | [ContainerInfo.LabelsEntry](#composia-agent-v1-ContainerInfo-LabelsEntry) | repeated | labels is the container label map. |
| ports | [string](#string) | repeated | ports contains display-formatted published port mappings. |
| networks | [string](#string) | repeated | networks lists connected Docker network names. |
| image_id | [string](#string) |  | image_id is the resolved image ID. |






<a name="composia-agent-v1-ContainerInfo-LabelsEntry"></a>

### ContainerInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="composia-agent-v1-DockerQueryTask"></a>

### DockerQueryTask
DockerQueryTask describes one synchronous Docker query assigned to an agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| query_id | [string](#string) |  | query_id is the controller-generated query ID. |
| node_id | [string](#string) |  | node_id is the target node ID for the query. |
| action | [string](#string) |  | action is one of list, inspect, or logs. |
| resource | [string](#string) |  | resource is one of container, containers, network, networks, volume, volumes, image, or images. |
| id | [string](#string) |  | id identifies the target Docker resource for inspect-like operations. |
| tail | [string](#string) |  | tail is forwarded to the Docker log tail option for log queries. |
| timestamps | [bool](#bool) |  | timestamps includes Docker timestamps in container log results. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size for list queries. |
| page | [uint32](#uint32) |  | page is the 1-based page number for list queries. |
| search | [string](#string) |  | search is the case-insensitive search term for list queries. |
| sort_by | [string](#string) |  | sort_by identifies the list sort field. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the list sort order when true. |






<a name="composia-agent-v1-DockerStats"></a>

### DockerStats
DockerStats describes a node-level Docker usage snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| containers_total | [uint32](#uint32) |  |  |
| containers_running | [uint32](#uint32) |  | containers_running is the number of running containers. |
| containers_stopped | [uint32](#uint32) |  | containers_stopped is the number of stopped containers. |
| containers_paused | [uint32](#uint32) |  | containers_paused is the number of paused containers. |
| images | [uint32](#uint32) |  | images is the number of local Docker images. |
| networks | [uint32](#uint32) |  | networks is the number of local Docker networks. |
| volumes | [uint32](#uint32) |  | volumes is the number of local Docker volumes. |
| volumes_size_bytes | [uint64](#uint64) |  | volumes_size_bytes is the total reported volume size in bytes. |
| disks_usage_bytes | [uint64](#uint64) |  | disks_usage_bytes is the Docker-reported disk usage in bytes. |
| docker_server_version | [string](#string) |  |  |






<a name="composia-agent-v1-GetContainerLogsRequest"></a>

### GetContainerLogsRequest
GetContainerLogsRequest fetches logs for one local container.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [string](#string) |  | container_id is the runtime container ID. |
| tail | [string](#string) |  | tail is forwarded to the Docker log tail option. |
| timestamps | [bool](#bool) |  | timestamps includes Docker timestamps in the returned log output. |






<a name="composia-agent-v1-GetContainerLogsResponse"></a>

### GetContainerLogsResponse
GetContainerLogsResponse returns one streamed log chunk.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| content | [string](#string) |  |  |






<a name="composia-agent-v1-GetServiceBundleRequest"></a>

### GetServiceBundleRequest
GetServiceBundleRequest identifies the task whose bundle should be streamed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID whose bundle should be streamed. |
| service_dir | [string](#string) |  | service_dir overrides the task service directory for multi-service bundle consumers. |






<a name="composia-agent-v1-GetServiceBundleResponse"></a>

### GetServiceBundleResponse
GetServiceBundleResponse carries one binary chunk from a task bundle stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| repo_revision | [string](#string) |  | repo_revision is the repo revision packaged in the bundle. |
| relative_root | [string](#string) |  | relative_root is the relative root path for the streamed bundle contents. |
| data | [bytes](#bytes) |  | data contains one chunk of the bundle payload. |






<a name="composia-agent-v1-HeartbeatRequest"></a>

### HeartbeatRequest
HeartbeatRequest is the lightweight keepalive sent by an agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| agent_version | [string](#string) |  | agent_version is the agent version string. |
| sent_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | sent_at is the agent time when the heartbeat was sent. |
| runtime | [NodeRuntimeSummary](#composia-agent-v1-NodeRuntimeSummary) |  |  |






<a name="composia-agent-v1-HeartbeatResponse"></a>

### HeartbeatResponse
HeartbeatResponse acknowledges a heartbeat with the controller receive time.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| received_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="composia-agent-v1-ImageInfo"></a>

### ImageInfo
ImageInfo describes one Docker image for list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| repo_tags | [string](#string) | repeated | repo_tags lists named tags referencing the image. |
| size | [int64](#int64) |  | size is the image size in bytes. |
| virtual_size | [int64](#int64) |  | virtual_size is the virtual image size in bytes. |
| created | [string](#string) |  | created is the image creation timestamp string. |
| repo_digests | [string](#string) | repeated | repo_digests lists immutable digest references for the image. |
| architecture | [string](#string) |  | architecture is the image CPU architecture string. |
| os | [string](#string) |  | os is the image operating system string. |
| author | [string](#string) |  | author is the image author metadata, when present. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of containers using the image. |
| is_dangling | [bool](#bool) |  | is_dangling reports whether the image is dangling. |






<a name="composia-agent-v1-InspectContainerRequest"></a>

### InspectContainerRequest
InspectContainerRequest identifies one local container.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [string](#string) |  | container_id is the runtime container ID. |






<a name="composia-agent-v1-InspectContainerResponse"></a>

### InspectContainerResponse
InspectContainerResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-agent-v1-InspectImageRequest"></a>

### InspectImageRequest
InspectImageRequest identifies one local image.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image_id | [string](#string) |  | image_id is the runtime image ID. |






<a name="composia-agent-v1-InspectImageResponse"></a>

### InspectImageResponse
InspectImageResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-agent-v1-InspectNetworkRequest"></a>

### InspectNetworkRequest
InspectNetworkRequest identifies one local network.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| network_id | [string](#string) |  | network_id is the runtime network ID. |






<a name="composia-agent-v1-InspectNetworkResponse"></a>

### InspectNetworkResponse
InspectNetworkResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-agent-v1-InspectVolumeRequest"></a>

### InspectVolumeRequest
InspectVolumeRequest identifies one local volume.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume_name | [string](#string) |  | volume_name is the runtime volume name. |






<a name="composia-agent-v1-InspectVolumeResponse"></a>

### InspectVolumeResponse
InspectVolumeResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-agent-v1-ListContainersRequest"></a>

### ListContainersRequest
ListContainersRequest requests all local Docker containers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-agent-v1-ListContainersResponse"></a>

### ListContainersResponse
ListContainersResponse returns local container summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| containers | [ContainerInfo](#composia-agent-v1-ContainerInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-agent-v1-ListImagesRequest"></a>

### ListImagesRequest
ListImagesRequest requests all local Docker images.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-agent-v1-ListImagesResponse"></a>

### ListImagesResponse
ListImagesResponse returns local image summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| images | [ImageInfo](#composia-agent-v1-ImageInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-agent-v1-ListNetworksRequest"></a>

### ListNetworksRequest
ListNetworksRequest requests all local Docker networks.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-agent-v1-ListNetworksResponse"></a>

### ListNetworksResponse
ListNetworksResponse returns local network summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| networks | [NetworkInfo](#composia-agent-v1-NetworkInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-agent-v1-ListVolumesRequest"></a>

### ListVolumesRequest
ListVolumesRequest requests all local Docker volumes.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-agent-v1-ListVolumesResponse"></a>

### ListVolumesResponse
ListVolumesResponse returns local volume summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volumes | [VolumeInfo](#composia-agent-v1-VolumeInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-agent-v1-NetworkInfo"></a>

### NetworkInfo
NetworkInfo describes one Docker network for list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | name is the runtime network name. |
| driver | [string](#string) |  | driver is the Docker network driver name. |
| scope | [string](#string) |  | scope is the Docker network scope. |
| internal | [bool](#bool) |  | internal reports whether the network is internal-only. |
| attachable | [bool](#bool) |  | attachable reports whether standalone containers may attach to the network. |
| created | [string](#string) |  | created is the network creation timestamp string. |
| labels | [NetworkInfo.LabelsEntry](#composia-agent-v1-NetworkInfo-LabelsEntry) | repeated | labels is the network label map. |
| subnet | [string](#string) |  | subnet is the primary IPv4 subnet, when available. |
| gateway | [string](#string) |  | gateway is the primary IPv4 gateway, when available. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of connected containers. |
| ipv6_enabled | [bool](#bool) |  | ipv6_enabled reports whether IPv6 is enabled for the network. |






<a name="composia-agent-v1-NetworkInfo-LabelsEntry"></a>

### NetworkInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="composia-agent-v1-NodeRuntimeSummary"></a>

### NodeRuntimeSummary
NodeRuntimeSummary reports basic runtime capacity for a node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| docker_server_version | [string](#string) |  | docker_server_version is the local Docker Engine version string. |
| disk_total_bytes | [uint64](#uint64) |  | disk_total_bytes is the total disk capacity visible to the agent. |
| disk_free_bytes | [uint64](#uint64) |  | disk_free_bytes is the currently free disk capacity visible to the agent. |






<a name="composia-agent-v1-OpenContainerLogTunnelRequest"></a>

### OpenContainerLogTunnelRequest
OpenContainerLogTunnelRequest carries one container log tunnel frame to the controller.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id identifies the container log session. |
| kind | [string](#string) |  | kind identifies the frame type carried in this message. |
| content | [string](#string) |  | content carries one log chunk for chunk frames. |
| error_message | [string](#string) |  | error_message contains the frame-level failure summary, when present. |
| error_code | [string](#string) |  | error_code carries a Connect-style lowercase code string such as not_found. |






<a name="composia-agent-v1-OpenContainerLogTunnelResponse"></a>

### OpenContainerLogTunnelResponse
OpenContainerLogTunnelResponse carries one container log tunnel frame to the agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id identifies the container log session. |
| kind | [string](#string) |  | kind identifies the frame type carried in this message. |
| container_id | [string](#string) |  | container_id identifies the target container for the log session. |
| tail | [string](#string) |  | tail is forwarded to the Docker log tail option for the session. |
| timestamps | [bool](#bool) |  | timestamps includes Docker timestamps in streamed log chunks. |






<a name="composia-agent-v1-OpenExecTunnelRequest"></a>

### OpenExecTunnelRequest
OpenExecTunnelRequest carries one frame for an interactive exec session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id identifies the interactive exec session. |
| kind | [string](#string) |  | kind identifies the frame type carried in this message. |
| payload | [bytes](#bytes) |  | payload holds raw terminal or control bytes for the frame. |
| rows | [uint32](#uint32) |  | rows is the terminal row count carried by resize frames. |
| cols | [uint32](#uint32) |  | cols is the terminal column count carried by resize frames. |
| node_id | [string](#string) |  | node_id identifies the node that should execute the command. |
| container_id | [string](#string) |  | container_id identifies the target container for the exec session. |
| command | [string](#string) | repeated | command stores the exec command and arguments. |






<a name="composia-agent-v1-OpenExecTunnelResponse"></a>

### OpenExecTunnelResponse
OpenExecTunnelResponse carries one frame back to the controller side.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id identifies the interactive exec session. |
| kind | [string](#string) |  | kind identifies the frame type carried in this message. |
| payload | [bytes](#bytes) |  | payload holds raw terminal or control bytes for the frame. |
| rows | [uint32](#uint32) |  | rows is the terminal row count carried by resize frames. |
| cols | [uint32](#uint32) |  | cols is the terminal column count carried by resize frames. |
| error | [string](#string) |  | error contains the frame-level error message, when present. |
| container_id | [string](#string) |  | container_id identifies the target container for the exec session. |
| command | [string](#string) | repeated | command stores the exec command and arguments. |






<a name="composia-agent-v1-PullNextDockerQueryRequest"></a>

### PullNextDockerQueryRequest
PullNextDockerQueryRequest identifies the node that is requesting a Docker query.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-agent-v1-PullNextDockerQueryResponse"></a>

### PullNextDockerQueryResponse
PullNextDockerQueryResponse indicates whether a Docker query was assigned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| has_query | [bool](#bool) |  | has_query is false when there is currently no Docker query to run. |
| query | [DockerQueryTask](#composia-agent-v1-DockerQueryTask) |  | query is populated only when has_query is true. |






<a name="composia-agent-v1-PullNextTaskRequest"></a>

### PullNextTaskRequest
PullNextTaskRequest identifies the node that is requesting work.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-agent-v1-PullNextTaskResponse"></a>

### PullNextTaskResponse
PullNextTaskResponse indicates whether a task was assigned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| has_task | [bool](#bool) |  | has_task is false when there is currently no task to run. |
| task | [AgentTask](#composia-agent-v1-AgentTask) |  | task is populated only when has_task is true. |






<a name="composia-agent-v1-RemoveContainerRequest"></a>

### RemoveContainerRequest
RemoveContainerRequest identifies one local container deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [string](#string) |  | container_id is the runtime container ID. |
| force | [bool](#bool) |  | force deletes the container even if it is running. |
| remove_volumes | [bool](#bool) |  | remove_volumes also removes anonymous volumes attached to the container. |






<a name="composia-agent-v1-RemoveContainerResponse"></a>

### RemoveContainerResponse
RemoveContainerResponse acknowledges one local container deletion request.






<a name="composia-agent-v1-RemoveImageRequest"></a>

### RemoveImageRequest
RemoveImageRequest identifies one local image deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image_id | [string](#string) |  | image_id is the runtime image ID or reference. |
| force | [bool](#bool) |  | force deletes the image even if it has multiple tags or dependent stopped containers. |






<a name="composia-agent-v1-RemoveImageResponse"></a>

### RemoveImageResponse
RemoveImageResponse acknowledges one local image deletion request.






<a name="composia-agent-v1-RemoveNetworkRequest"></a>

### RemoveNetworkRequest
RemoveNetworkRequest identifies one local network deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| network_id | [string](#string) |  | network_id is the runtime network ID. |






<a name="composia-agent-v1-RemoveNetworkResponse"></a>

### RemoveNetworkResponse
RemoveNetworkResponse acknowledges one local network deletion request.






<a name="composia-agent-v1-RemoveVolumeRequest"></a>

### RemoveVolumeRequest
RemoveVolumeRequest identifies one local volume deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume_name | [string](#string) |  | volume_name is the runtime volume name. |






<a name="composia-agent-v1-RemoveVolumeResponse"></a>

### RemoveVolumeResponse
RemoveVolumeResponse acknowledges one local volume deletion request.






<a name="composia-agent-v1-ReportBackupResultRequest"></a>

### ReportBackupResultRequest
ReportBackupResultRequest reports the result of one backup execution.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup_id | [string](#string) |  | backup_id is the backup record ID being updated. |
| task_id | [string](#string) |  | task_id is the controller task ID that triggered the backup. |
| service_name | [string](#string) |  | service_name is the logical service name. |
| data_name | [string](#string) |  | data_name is the service data entry that was backed up. |
| status | [string](#string) |  | status is the latest backup status string observed by the agent. |
| started_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_at is the backup start time. |
| finished_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_at is set when the backup reaches a terminal state. |
| artifact_ref | [string](#string) |  | artifact_ref identifies the produced backup artifact, when present. |
| error_summary | [string](#string) |  | error_summary contains the failure summary when the backup fails. |






<a name="composia-agent-v1-ReportBackupResultResponse"></a>

### ReportBackupResultResponse
ReportBackupResultResponse acknowledges a backup result update.






<a name="composia-agent-v1-ReportDockerQueryResultRequest"></a>

### ReportDockerQueryResultRequest
ReportDockerQueryResultRequest reports the result of one direct Docker query.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| query_id | [string](#string) |  | query_id is the controller-generated query ID. |
| node_id | [string](#string) |  | node_id is the stable node identifier that executed the query. |
| payload_json | [string](#string) |  | payload_json carries the JSON-encoded query payload on success. |
| error_message | [string](#string) |  | error_message contains the human-readable failure summary when the query fails. |
| error_code | [string](#string) |  | error_code carries a Connect-style lowercase code string such as not_found. |






<a name="composia-agent-v1-ReportDockerQueryResultResponse"></a>

### ReportDockerQueryResultResponse
ReportDockerQueryResultResponse acknowledges a Docker query result.






<a name="composia-agent-v1-ReportDockerStatsRequest"></a>

### ReportDockerStatsRequest
ReportDockerStatsRequest reports the latest node Docker stats snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| stats | [DockerStats](#composia-agent-v1-DockerStats) |  | stats is the current Docker usage snapshot for the node. |






<a name="composia-agent-v1-ReportDockerStatsResponse"></a>

### ReportDockerStatsResponse
ReportDockerStatsResponse acknowledges a Docker stats update.






<a name="composia-agent-v1-ReportServiceInstanceStatusRequest"></a>

### ReportServiceInstanceStatusRequest
ReportServiceInstanceStatusRequest reports one service instance runtime snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| runtime_status | [string](#string) |  | runtime_status is the latest instance status string observed by the agent. |
| reported_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | reported_at is the time when the status snapshot was observed. |






<a name="composia-agent-v1-ReportServiceInstanceStatusResponse"></a>

### ReportServiceInstanceStatusResponse
ReportServiceInstanceStatusResponse acknowledges a service status update.






<a name="composia-agent-v1-ReportTaskStateRequest"></a>

### ReportTaskStateRequest
ReportTaskStateRequest reports the latest task-level state from an agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID being updated. |
| status | [string](#string) |  | status is the latest task status string observed by the agent. |
| error_summary | [string](#string) |  | error_summary contains the task failure summary when the task fails. |
| finished_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_at is set when the task reaches a terminal state. |






<a name="composia-agent-v1-ReportTaskStateResponse"></a>

### ReportTaskStateResponse
ReportTaskStateResponse acknowledges a task state update.






<a name="composia-agent-v1-ReportTaskStepStateRequest"></a>

### ReportTaskStepStateRequest
ReportTaskStepStateRequest reports the latest task step state from an agent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID being updated. |
| step_name | [string](#string) |  | step_name identifies the step being updated. |
| status | [string](#string) |  | status is the latest step status string observed by the agent. |
| started_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_at is set when the step begins. |
| finished_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_at is set when the step reaches a terminal state. |






<a name="composia-agent-v1-ReportTaskStepStateResponse"></a>

### ReportTaskStepStateResponse
ReportTaskStepStateResponse acknowledges a task step state update.






<a name="composia-agent-v1-RunContainerActionRequest"></a>

### RunContainerActionRequest
RunContainerActionRequest identifies one local container action.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [string](#string) |  |  |
| action | [ContainerAction](#composia-agent-v1-ContainerAction) |  |  |






<a name="composia-agent-v1-RunContainerActionResponse"></a>

### RunContainerActionResponse
RunContainerActionResponse acknowledges a local container action request.






<a name="composia-agent-v1-UploadTaskLogsRequest"></a>

### UploadTaskLogsRequest
UploadTaskLogsRequest carries one ordered log chunk for a task.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID receiving the log chunk. |
| seq | [uint64](#uint64) |  | seq is a monotonically increasing sequence number per task log stream. |
| sent_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | sent_at is the agent time when this log chunk was sent. |
| content | [string](#string) |  | content is the incremental log payload. |






<a name="composia-agent-v1-UploadTaskLogsResponse"></a>

### UploadTaskLogsResponse
UploadTaskLogsResponse acknowledges received task log chunks.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  | task_id is the controller task ID receiving the acknowledgement. |
| last_confirmed_seq | [uint64](#uint64) |  | last_confirmed_seq is the last sequence the controller accepted. |






<a name="composia-agent-v1-VolumeInfo"></a>

### VolumeInfo
VolumeInfo describes one Docker volume for list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the runtime volume name. |
| driver | [string](#string) |  | driver is the Docker volume driver name. |
| mountpoint | [string](#string) |  | mountpoint is the volume mount path on the node. |
| scope | [string](#string) |  | scope is the Docker volume scope. |
| created | [string](#string) |  | created is the volume creation timestamp string. |
| labels | [VolumeInfo.LabelsEntry](#composia-agent-v1-VolumeInfo-LabelsEntry) | repeated | labels is the volume label map. |
| size_bytes | [int64](#int64) |  | size_bytes is the reported volume size in bytes. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of attached containers. |
| in_use | [bool](#bool) |  | in_use reports whether any container currently uses the volume. |






<a name="composia-agent-v1-VolumeInfo-LabelsEntry"></a>

### VolumeInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 


<a name="composia-agent-v1-ContainerAction"></a>

### ContainerAction
ContainerAction identifies a container lifecycle action.

| Name | Number | Description |
| ---- | ------ | ----------- |
| CONTAINER_ACTION_UNSPECIFIED | 0 | CONTAINER_ACTION_UNSPECIFIED is invalid and should not be used. |
| CONTAINER_ACTION_START | 1 | CONTAINER_ACTION_START starts the container. |
| CONTAINER_ACTION_STOP | 2 | CONTAINER_ACTION_STOP stops the container. |
| CONTAINER_ACTION_RESTART | 3 | CONTAINER_ACTION_RESTART restarts the container. |


 

 


<a name="composia-agent-v1-AgentReportService"></a>

### AgentReportService
AgentReportService carries agent-to-controller runtime and task reports.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Heartbeat | [HeartbeatRequest](#composia-agent-v1-HeartbeatRequest) | [HeartbeatResponse](#composia-agent-v1-HeartbeatResponse) | Heartbeat reports that an agent is alive and includes runtime metadata. |
| ReportTaskState | [ReportTaskStateRequest](#composia-agent-v1-ReportTaskStateRequest) | [ReportTaskStateResponse](#composia-agent-v1-ReportTaskStateResponse) | ReportTaskState reports the latest state for one task. |
| ReportTaskStepState | [ReportTaskStepStateRequest](#composia-agent-v1-ReportTaskStepStateRequest) | [ReportTaskStepStateResponse](#composia-agent-v1-ReportTaskStepStateResponse) | ReportTaskStepState reports the latest state for one task step. |
| UploadTaskLogs | [UploadTaskLogsRequest](#composia-agent-v1-UploadTaskLogsRequest) stream | [UploadTaskLogsResponse](#composia-agent-v1-UploadTaskLogsResponse) stream | UploadTaskLogs streams log chunks from the agent to the controller. |
| ReportBackupResult | [ReportBackupResultRequest](#composia-agent-v1-ReportBackupResultRequest) | [ReportBackupResultResponse](#composia-agent-v1-ReportBackupResultResponse) | ReportBackupResult reports the result of one backup operation. |
| ReportServiceInstanceStatus | [ReportServiceInstanceStatusRequest](#composia-agent-v1-ReportServiceInstanceStatusRequest) | [ReportServiceInstanceStatusResponse](#composia-agent-v1-ReportServiceInstanceStatusResponse) | ReportServiceInstanceStatus reports the current status of one service instance. |
| ReportDockerStats | [ReportDockerStatsRequest](#composia-agent-v1-ReportDockerStatsRequest) | [ReportDockerStatsResponse](#composia-agent-v1-ReportDockerStatsResponse) | ReportDockerStats reports the latest Docker stats snapshot for one node. |
| ReportDockerQueryResult | [ReportDockerQueryResultRequest](#composia-agent-v1-ReportDockerQueryResultRequest) | [ReportDockerQueryResultResponse](#composia-agent-v1-ReportDockerQueryResultResponse) | ReportDockerQueryResult reports the result of one direct Docker query. |
| OpenExecTunnel | [OpenExecTunnelRequest](#composia-agent-v1-OpenExecTunnelRequest) stream | [OpenExecTunnelResponse](#composia-agent-v1-OpenExecTunnelResponse) stream | OpenExecTunnel proxies interactive exec traffic between controller and agent. |
| OpenContainerLogTunnel | [OpenContainerLogTunnelRequest](#composia-agent-v1-OpenContainerLogTunnelRequest) stream | [OpenContainerLogTunnelResponse](#composia-agent-v1-OpenContainerLogTunnelResponse) stream | OpenContainerLogTunnel proxies live container log traffic between controller and agent. |


<a name="composia-agent-v1-AgentTaskService"></a>

### AgentTaskService
AgentTaskService lets an agent pull work assigned to its node.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| PullNextTask | [PullNextTaskRequest](#composia-agent-v1-PullNextTaskRequest) | [PullNextTaskResponse](#composia-agent-v1-PullNextTaskResponse) | PullNextTask returns the next available task for the requesting node. |
| PullNextDockerQuery | [PullNextDockerQueryRequest](#composia-agent-v1-PullNextDockerQueryRequest) | [PullNextDockerQueryResponse](#composia-agent-v1-PullNextDockerQueryResponse) | PullNextDockerQuery returns the next in-memory Docker query for the requesting node. |


<a name="composia-agent-v1-BundleService"></a>

### BundleService
BundleService streams service bundles needed to execute a task.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetServiceBundle | [GetServiceBundleRequest](#composia-agent-v1-GetServiceBundleRequest) | [GetServiceBundleResponse](#composia-agent-v1-GetServiceBundleResponse) stream | GetServiceBundle streams the task bundle as binary chunks. |


<a name="composia-agent-v1-DockerService"></a>

### DockerService
DockerService exposes Docker operations that run on the agent node.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListContainers | [ListContainersRequest](#composia-agent-v1-ListContainersRequest) | [ListContainersResponse](#composia-agent-v1-ListContainersResponse) | ListContainers lists local Docker containers. |
| InspectContainer | [InspectContainerRequest](#composia-agent-v1-InspectContainerRequest) | [InspectContainerResponse](#composia-agent-v1-InspectContainerResponse) | InspectContainer returns raw Docker inspect JSON for one container. |
| RunContainerAction | [RunContainerActionRequest](#composia-agent-v1-RunContainerActionRequest) | [RunContainerActionResponse](#composia-agent-v1-RunContainerActionResponse) | RunContainerAction applies a lifecycle action to one container. |
| RemoveContainer | [RemoveContainerRequest](#composia-agent-v1-RemoveContainerRequest) | [RemoveContainerResponse](#composia-agent-v1-RemoveContainerResponse) | RemoveContainer deletes one container. |
| GetContainerLogs | [GetContainerLogsRequest](#composia-agent-v1-GetContainerLogsRequest) | [GetContainerLogsResponse](#composia-agent-v1-GetContainerLogsResponse) stream | GetContainerLogs streams log text for one container. |
| ListNetworks | [ListNetworksRequest](#composia-agent-v1-ListNetworksRequest) | [ListNetworksResponse](#composia-agent-v1-ListNetworksResponse) | ListNetworks lists local Docker networks. |
| InspectNetwork | [InspectNetworkRequest](#composia-agent-v1-InspectNetworkRequest) | [InspectNetworkResponse](#composia-agent-v1-InspectNetworkResponse) | InspectNetwork returns raw Docker inspect JSON for one network. |
| RemoveNetwork | [RemoveNetworkRequest](#composia-agent-v1-RemoveNetworkRequest) | [RemoveNetworkResponse](#composia-agent-v1-RemoveNetworkResponse) | RemoveNetwork deletes one network. |
| ListVolumes | [ListVolumesRequest](#composia-agent-v1-ListVolumesRequest) | [ListVolumesResponse](#composia-agent-v1-ListVolumesResponse) | ListVolumes lists local Docker volumes. |
| InspectVolume | [InspectVolumeRequest](#composia-agent-v1-InspectVolumeRequest) | [InspectVolumeResponse](#composia-agent-v1-InspectVolumeResponse) | InspectVolume returns raw Docker inspect JSON for one volume. |
| RemoveVolume | [RemoveVolumeRequest](#composia-agent-v1-RemoveVolumeRequest) | [RemoveVolumeResponse](#composia-agent-v1-RemoveVolumeResponse) | RemoveVolume deletes one volume. |
| ListImages | [ListImagesRequest](#composia-agent-v1-ListImagesRequest) | [ListImagesResponse](#composia-agent-v1-ListImagesResponse) | ListImages lists local Docker images. |
| InspectImage | [InspectImageRequest](#composia-agent-v1-InspectImageRequest) | [InspectImageResponse](#composia-agent-v1-InspectImageResponse) | InspectImage returns raw Docker inspect JSON for one image. |
| RemoveImage | [RemoveImageRequest](#composia-agent-v1-RemoveImageRequest) | [RemoveImageResponse](#composia-agent-v1-RemoveImageResponse) | RemoveImage deletes one image. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

