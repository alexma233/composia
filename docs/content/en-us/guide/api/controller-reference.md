# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [proto/composia/controller/v1/task.proto](#proto_composia_controller_v1_task-proto)
    - [GetTaskRequest](#composia-controller-v1-GetTaskRequest)
    - [GetTaskResponse](#composia-controller-v1-GetTaskResponse)
    - [ListTasksRequest](#composia-controller-v1-ListTasksRequest)
    - [ListTasksResponse](#composia-controller-v1-ListTasksResponse)
    - [ResolveTaskConfirmationRequest](#composia-controller-v1-ResolveTaskConfirmationRequest)
    - [RunTaskAgainRequest](#composia-controller-v1-RunTaskAgainRequest)
    - [TailTaskLogsRequest](#composia-controller-v1-TailTaskLogsRequest)
    - [TailTaskLogsResponse](#composia-controller-v1-TailTaskLogsResponse)
    - [TaskActionResponse](#composia-controller-v1-TaskActionResponse)
    - [TaskStepSummary](#composia-controller-v1-TaskStepSummary)
    - [TaskSummary](#composia-controller-v1-TaskSummary)
  
    - [TaskService](#composia-controller-v1-TaskService)
  
- [proto/composia/controller/v1/system.proto](#proto_composia_controller_v1_system-proto)
    - [AccessTokenSummary](#composia-controller-v1-AccessTokenSummary)
    - [BackupConfigSummary](#composia-controller-v1-BackupConfigSummary)
    - [Capability](#composia-controller-v1-Capability)
    - [DNSConfigSummary](#composia-controller-v1-DNSConfigSummary)
    - [GetCapabilitiesRequest](#composia-controller-v1-GetCapabilitiesRequest)
    - [GetCapabilitiesResponse](#composia-controller-v1-GetCapabilitiesResponse)
    - [GetCurrentConfigRequest](#composia-controller-v1-GetCurrentConfigRequest)
    - [GetCurrentConfigResponse](#composia-controller-v1-GetCurrentConfigResponse)
    - [GetSystemStatusRequest](#composia-controller-v1-GetSystemStatusRequest)
    - [GetSystemStatusResponse](#composia-controller-v1-GetSystemStatusResponse)
    - [GitConfigSummary](#composia-controller-v1-GitConfigSummary)
    - [GlobalCapabilities](#composia-controller-v1-GlobalCapabilities)
    - [NodeConfigSummary](#composia-controller-v1-NodeConfigSummary)
    - [ReloadControllerConfigRequest](#composia-controller-v1-ReloadControllerConfigRequest)
    - [ReloadControllerConfigResponse](#composia-controller-v1-ReloadControllerConfigResponse)
    - [SecretsConfigSummary](#composia-controller-v1-SecretsConfigSummary)
  
    - [SystemService](#composia-controller-v1-SystemService)
  
- [proto/composia/controller/v1/backup.proto](#proto_composia_controller_v1_backup-proto)
    - [BackupActionCapabilities](#composia-controller-v1-BackupActionCapabilities)
    - [BackupSummary](#composia-controller-v1-BackupSummary)
    - [GetBackupRequest](#composia-controller-v1-GetBackupRequest)
    - [GetBackupResponse](#composia-controller-v1-GetBackupResponse)
    - [ListBackupsRequest](#composia-controller-v1-ListBackupsRequest)
    - [ListBackupsResponse](#composia-controller-v1-ListBackupsResponse)
    - [RestoreBackupRequest](#composia-controller-v1-RestoreBackupRequest)
  
    - [BackupRecordService](#composia-controller-v1-BackupRecordService)
  
- [proto/composia/controller/v1/container.proto](#proto_composia_controller_v1_container-proto)
    - [GetContainerLogsRequest](#composia-controller-v1-GetContainerLogsRequest)
    - [GetContainerLogsResponse](#composia-controller-v1-GetContainerLogsResponse)
    - [OpenContainerExecRequest](#composia-controller-v1-OpenContainerExecRequest)
    - [OpenContainerExecResponse](#composia-controller-v1-OpenContainerExecResponse)
    - [RemoveContainerRequest](#composia-controller-v1-RemoveContainerRequest)
    - [RemoveImageRequest](#composia-controller-v1-RemoveImageRequest)
    - [RemoveNetworkRequest](#composia-controller-v1-RemoveNetworkRequest)
    - [RemoveVolumeRequest](#composia-controller-v1-RemoveVolumeRequest)
    - [RunContainerActionRequest](#composia-controller-v1-RunContainerActionRequest)
  
    - [ContainerAction](#composia-controller-v1-ContainerAction)
  
    - [ContainerService](#composia-controller-v1-ContainerService)
  
- [proto/composia/controller/v1/node.proto](#proto_composia_controller_v1_node-proto)
    - [ContainerInfo](#composia-controller-v1-ContainerInfo)
    - [ContainerInfo.LabelsEntry](#composia-controller-v1-ContainerInfo-LabelsEntry)
    - [DockerStats](#composia-controller-v1-DockerStats)
    - [ForgetNodeRusticRequest](#composia-controller-v1-ForgetNodeRusticRequest)
    - [ForgetNodeRusticResponse](#composia-controller-v1-ForgetNodeRusticResponse)
    - [GetNodeDockerStatsRequest](#composia-controller-v1-GetNodeDockerStatsRequest)
    - [GetNodeDockerStatsResponse](#composia-controller-v1-GetNodeDockerStatsResponse)
    - [GetNodeRequest](#composia-controller-v1-GetNodeRequest)
    - [GetNodeResponse](#composia-controller-v1-GetNodeResponse)
    - [GetNodeTasksRequest](#composia-controller-v1-GetNodeTasksRequest)
    - [GetNodeTasksResponse](#composia-controller-v1-GetNodeTasksResponse)
    - [ImageInfo](#composia-controller-v1-ImageInfo)
    - [InitNodeRusticRequest](#composia-controller-v1-InitNodeRusticRequest)
    - [InitNodeRusticResponse](#composia-controller-v1-InitNodeRusticResponse)
    - [InspectNodeContainerRequest](#composia-controller-v1-InspectNodeContainerRequest)
    - [InspectNodeContainerResponse](#composia-controller-v1-InspectNodeContainerResponse)
    - [InspectNodeImageRequest](#composia-controller-v1-InspectNodeImageRequest)
    - [InspectNodeImageResponse](#composia-controller-v1-InspectNodeImageResponse)
    - [InspectNodeNetworkRequest](#composia-controller-v1-InspectNodeNetworkRequest)
    - [InspectNodeNetworkResponse](#composia-controller-v1-InspectNodeNetworkResponse)
    - [InspectNodeVolumeRequest](#composia-controller-v1-InspectNodeVolumeRequest)
    - [InspectNodeVolumeResponse](#composia-controller-v1-InspectNodeVolumeResponse)
    - [ListNodeContainersRequest](#composia-controller-v1-ListNodeContainersRequest)
    - [ListNodeContainersResponse](#composia-controller-v1-ListNodeContainersResponse)
    - [ListNodeImagesRequest](#composia-controller-v1-ListNodeImagesRequest)
    - [ListNodeImagesResponse](#composia-controller-v1-ListNodeImagesResponse)
    - [ListNodeNetworksRequest](#composia-controller-v1-ListNodeNetworksRequest)
    - [ListNodeNetworksResponse](#composia-controller-v1-ListNodeNetworksResponse)
    - [ListNodeVolumesRequest](#composia-controller-v1-ListNodeVolumesRequest)
    - [ListNodeVolumesResponse](#composia-controller-v1-ListNodeVolumesResponse)
    - [ListNodesRequest](#composia-controller-v1-ListNodesRequest)
    - [ListNodesResponse](#composia-controller-v1-ListNodesResponse)
    - [NetworkInfo](#composia-controller-v1-NetworkInfo)
    - [NetworkInfo.LabelsEntry](#composia-controller-v1-NetworkInfo-LabelsEntry)
    - [NodeActionCapabilities](#composia-controller-v1-NodeActionCapabilities)
    - [NodeSummary](#composia-controller-v1-NodeSummary)
    - [PruneNodeDockerRequest](#composia-controller-v1-PruneNodeDockerRequest)
    - [PruneNodeDockerResponse](#composia-controller-v1-PruneNodeDockerResponse)
    - [PruneNodeRusticRequest](#composia-controller-v1-PruneNodeRusticRequest)
    - [PruneNodeRusticResponse](#composia-controller-v1-PruneNodeRusticResponse)
    - [ReloadNodeCaddyRequest](#composia-controller-v1-ReloadNodeCaddyRequest)
    - [ReloadNodeCaddyResponse](#composia-controller-v1-ReloadNodeCaddyResponse)
    - [SyncNodeCaddyFilesRequest](#composia-controller-v1-SyncNodeCaddyFilesRequest)
    - [SyncNodeCaddyFilesResponse](#composia-controller-v1-SyncNodeCaddyFilesResponse)
    - [VolumeInfo](#composia-controller-v1-VolumeInfo)
    - [VolumeInfo.LabelsEntry](#composia-controller-v1-VolumeInfo-LabelsEntry)
  
    - [DockerQueryService](#composia-controller-v1-DockerQueryService)
    - [NodeMaintenanceService](#composia-controller-v1-NodeMaintenanceService)
    - [NodeQueryService](#composia-controller-v1-NodeQueryService)
  
- [proto/composia/controller/v1/repo.proto](#proto_composia_controller_v1_repo-proto)
    - [CreateRepoDirectoryRequest](#composia-controller-v1-CreateRepoDirectoryRequest)
    - [CreateRepoDirectoryResponse](#composia-controller-v1-CreateRepoDirectoryResponse)
    - [DeleteRepoPathRequest](#composia-controller-v1-DeleteRepoPathRequest)
    - [DeleteRepoPathResponse](#composia-controller-v1-DeleteRepoPathResponse)
    - [GetRepoFileRequest](#composia-controller-v1-GetRepoFileRequest)
    - [GetRepoFileResponse](#composia-controller-v1-GetRepoFileResponse)
    - [GetRepoHeadRequest](#composia-controller-v1-GetRepoHeadRequest)
    - [GetRepoHeadResponse](#composia-controller-v1-GetRepoHeadResponse)
    - [ListRepoCommitsRequest](#composia-controller-v1-ListRepoCommitsRequest)
    - [ListRepoCommitsResponse](#composia-controller-v1-ListRepoCommitsResponse)
    - [ListRepoFilesRequest](#composia-controller-v1-ListRepoFilesRequest)
    - [ListRepoFilesResponse](#composia-controller-v1-ListRepoFilesResponse)
    - [MoveRepoPathRequest](#composia-controller-v1-MoveRepoPathRequest)
    - [MoveRepoPathResponse](#composia-controller-v1-MoveRepoPathResponse)
    - [RepoCommitSummary](#composia-controller-v1-RepoCommitSummary)
    - [RepoFileEntry](#composia-controller-v1-RepoFileEntry)
    - [RepoValidationError](#composia-controller-v1-RepoValidationError)
    - [SyncRepoRequest](#composia-controller-v1-SyncRepoRequest)
    - [SyncRepoResponse](#composia-controller-v1-SyncRepoResponse)
    - [UpdateRepoFileRequest](#composia-controller-v1-UpdateRepoFileRequest)
    - [UpdateRepoFileResponse](#composia-controller-v1-UpdateRepoFileResponse)
    - [ValidateRepoRequest](#composia-controller-v1-ValidateRepoRequest)
    - [ValidateRepoResponse](#composia-controller-v1-ValidateRepoResponse)
  
    - [RepoCommandService](#composia-controller-v1-RepoCommandService)
    - [RepoQueryService](#composia-controller-v1-RepoQueryService)
  
- [proto/composia/controller/v1/secret.proto](#proto_composia_controller_v1_secret-proto)
    - [GetSecretRequest](#composia-controller-v1-GetSecretRequest)
    - [GetSecretResponse](#composia-controller-v1-GetSecretResponse)
    - [UpdateSecretRequest](#composia-controller-v1-UpdateSecretRequest)
    - [UpdateSecretResponse](#composia-controller-v1-UpdateSecretResponse)
  
    - [SecretService](#composia-controller-v1-SecretService)
  
- [proto/composia/controller/v1/service.proto](#proto_composia_controller_v1_service-proto)
    - [GetServiceBackupsRequest](#composia-controller-v1-GetServiceBackupsRequest)
    - [GetServiceBackupsResponse](#composia-controller-v1-GetServiceBackupsResponse)
    - [GetServiceInstanceRequest](#composia-controller-v1-GetServiceInstanceRequest)
    - [GetServiceInstanceResponse](#composia-controller-v1-GetServiceInstanceResponse)
    - [GetServiceRequest](#composia-controller-v1-GetServiceRequest)
    - [GetServiceResponse](#composia-controller-v1-GetServiceResponse)
    - [GetServiceTasksRequest](#composia-controller-v1-GetServiceTasksRequest)
    - [GetServiceTasksResponse](#composia-controller-v1-GetServiceTasksResponse)
    - [GetServiceWorkspaceRequest](#composia-controller-v1-GetServiceWorkspaceRequest)
    - [GetServiceWorkspaceResponse](#composia-controller-v1-GetServiceWorkspaceResponse)
    - [ListServiceInstancesRequest](#composia-controller-v1-ListServiceInstancesRequest)
    - [ListServiceInstancesResponse](#composia-controller-v1-ListServiceInstancesResponse)
    - [ListServiceWorkspacesRequest](#composia-controller-v1-ListServiceWorkspacesRequest)
    - [ListServiceWorkspacesResponse](#composia-controller-v1-ListServiceWorkspacesResponse)
    - [ListServicesRequest](#composia-controller-v1-ListServicesRequest)
    - [ListServicesResponse](#composia-controller-v1-ListServicesResponse)
    - [MigrateServiceRequest](#composia-controller-v1-MigrateServiceRequest)
    - [RunServiceActionRequest](#composia-controller-v1-RunServiceActionRequest)
    - [RunServiceInstanceActionRequest](#composia-controller-v1-RunServiceInstanceActionRequest)
    - [ServiceActionCapabilities](#composia-controller-v1-ServiceActionCapabilities)
    - [ServiceContainerSummary](#composia-controller-v1-ServiceContainerSummary)
    - [ServiceInstanceDetail](#composia-controller-v1-ServiceInstanceDetail)
    - [ServiceInstanceSummary](#composia-controller-v1-ServiceInstanceSummary)
    - [ServiceSummary](#composia-controller-v1-ServiceSummary)
    - [ServiceWorkspaceSummary](#composia-controller-v1-ServiceWorkspaceSummary)
    - [UpdateServiceTargetNodesRequest](#composia-controller-v1-UpdateServiceTargetNodesRequest)
    - [UpdateServiceTargetNodesResponse](#composia-controller-v1-UpdateServiceTargetNodesResponse)
  
    - [ServiceAction](#composia-controller-v1-ServiceAction)
    - [ServiceInstanceAction](#composia-controller-v1-ServiceInstanceAction)
  
    - [ServiceCommandService](#composia-controller-v1-ServiceCommandService)
    - [ServiceInstanceService](#composia-controller-v1-ServiceInstanceService)
    - [ServiceQueryService](#composia-controller-v1-ServiceQueryService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="proto_composia_controller_v1_task-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/task.proto



<a name="composia-controller-v1-GetTaskRequest"></a>

### GetTaskRequest
GetTaskRequest identifies one task by ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-GetTaskResponse"></a>

### GetTaskResponse
GetTaskResponse describes one task, including step state and log metadata.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |
| type | [string](#string) |  | type is the controller task type string. |
| source | [string](#string) |  | source identifies what triggered the task. |
| service_name | [string](#string) |  |  |
| node_id | [string](#string) |  |  |
| status | [string](#string) |  | status is the latest task status string. |
| created_at | [string](#string) |  | created_at is the task creation timestamp string. |
| started_at | [string](#string) |  | started_at is empty until task execution begins. |
| finished_at | [string](#string) |  | finished_at is empty until task execution reaches a terminal state. |
| repo_revision | [string](#string) |  | repo_revision is the repo revision used when the task started. |
| error_summary | [string](#string) |  |  |
| log_path | [string](#string) |  | log_path is the controller-side path to persisted task logs. |
| steps | [TaskStepSummary](#composia-controller-v1-TaskStepSummary) | repeated | steps lists recorded step state snapshots in execution order. |
| triggered_by | [string](#string) |  | triggered_by identifies the actor that created the task. |
| result_revision | [string](#string) |  | result_revision is the repo revision produced by the task, when applicable. |
| attempt_of_task_id | [string](#string) |  | attempt_of_task_id links this task to the prior task it retried. |






<a name="composia-controller-v1-ListTasksRequest"></a>

### ListTasksRequest
ListTasksRequest filters task results by included and excluded values.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [string](#string) | repeated | status includes only tasks matching one of these status strings. |
| service_name | [string](#string) | repeated | service_name includes only tasks for these services. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| node_id | [string](#string) | repeated | node_id includes only tasks for these nodes. |
| type | [string](#string) | repeated | type includes only tasks of these types. |
| exclude_status | [string](#string) | repeated | exclude_status removes tasks matching these status strings. |
| exclude_service_name | [string](#string) | repeated | exclude_service_name removes tasks for these services. |
| exclude_node_id | [string](#string) | repeated | exclude_node_id removes tasks for these nodes. |
| exclude_type | [string](#string) | repeated | exclude_type removes tasks of these types. |






<a name="composia-controller-v1-ListTasksResponse"></a>

### ListTasksResponse
ListTasksResponse returns one page of task summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tasks | [TaskSummary](#composia-controller-v1-TaskSummary) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-ResolveTaskConfirmationRequest"></a>

### ResolveTaskConfirmationRequest
ResolveTaskConfirmationRequest resolves a task in awaiting_confirmation state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |
| decision | [string](#string) |  | decision accepts &#34;approve&#34; or &#34;reject&#34;. |
| comment | [string](#string) |  |  |






<a name="composia-controller-v1-RunTaskAgainRequest"></a>

### RunTaskAgainRequest
RunTaskAgainRequest identifies the task to retry as a new task.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-TailTaskLogsRequest"></a>

### TailTaskLogsRequest
TailTaskLogsRequest identifies the task log stream to follow.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-TailTaskLogsResponse"></a>

### TailTaskLogsResponse
TailTaskLogsResponse carries one incremental log chunk.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| content | [string](#string) |  |  |






<a name="composia-controller-v1-TaskActionResponse"></a>

### TaskActionResponse
TaskActionResponse reports the async task created by a command RPC.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |
| status | [string](#string) |  | status is the initial status of the created task. |
| repo_revision | [string](#string) |  | repo_revision is the repo revision associated with the created task. |






<a name="composia-controller-v1-TaskStepSummary"></a>

### TaskStepSummary
TaskStepSummary describes one recorded step within a task.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| step_name | [string](#string) |  |  |
| status | [string](#string) |  | status is the latest step status string. |
| started_at | [string](#string) |  | started_at is empty when the step has not started. |
| finished_at | [string](#string) |  | finished_at is empty until the step finishes. |






<a name="composia-controller-v1-TaskSummary"></a>

### TaskSummary
TaskSummary describes one task in list results.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |
| type | [string](#string) |  | type is the controller task type string. |
| status | [string](#string) |  | status is the latest task status string. |
| service_name | [string](#string) |  |  |
| node_id | [string](#string) |  |  |
| created_at | [string](#string) |  | created_at is the task creation timestamp string. |





 

 

 


<a name="composia-controller-v1-TaskService"></a>

### TaskService
TaskService exposes task queries, log streaming, retry operations, and confirmation resolution.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListTasks | [ListTasksRequest](#composia-controller-v1-ListTasksRequest) | [ListTasksResponse](#composia-controller-v1-ListTasksResponse) | ListTasks returns task summaries using include and exclude filters. |
| GetTask | [GetTaskRequest](#composia-controller-v1-GetTaskRequest) | [GetTaskResponse](#composia-controller-v1-GetTaskResponse) | GetTask returns the full detail for one task. |
| TailTaskLogs | [TailTaskLogsRequest](#composia-controller-v1-TailTaskLogsRequest) | [TailTaskLogsResponse](#composia-controller-v1-TailTaskLogsResponse) stream | TailTaskLogs streams incremental log content for one task. |
| RunTaskAgain | [RunTaskAgainRequest](#composia-controller-v1-RunTaskAgainRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RunTaskAgain starts a new task based on an existing task. |
| ResolveTaskConfirmation | [ResolveTaskConfirmationRequest](#composia-controller-v1-ResolveTaskConfirmationRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | ResolveTaskConfirmation resumes or rejects a task waiting for manual confirmation. |

 



<a name="proto_composia_controller_v1_system-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/system.proto



<a name="composia-controller-v1-AccessTokenSummary"></a>

### AccessTokenSummary
AccessTokenSummary describes one controller access token without exposing the token string.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the operator-facing token name. |
| enabled | [bool](#bool) |  | enabled reports whether this token may be used. |
| comment | [string](#string) |  | comment is the optional operator note attached to the token. |






<a name="composia-controller-v1-BackupConfigSummary"></a>

### BackupConfigSummary
BackupConfigSummary describes optional backup integration state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| has_rustic | [bool](#bool) |  | has_rustic reports whether Rustic backup integration is configured. |






<a name="composia-controller-v1-Capability"></a>

### Capability
Capability describes whether a feature or action may currently be used.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| enabled | [bool](#bool) |  |  |
| reason_code | [string](#string) |  | reason_code explains why the capability is disabled when enabled is false. |






<a name="composia-controller-v1-DNSConfigSummary"></a>

### DNSConfigSummary
DNSConfigSummary describes optional DNS integration state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| has_cloudflare | [bool](#bool) |  | has_cloudflare reports whether Cloudflare DNS integration is configured. |






<a name="composia-controller-v1-GetCapabilitiesRequest"></a>

### GetCapabilitiesRequest
GetCapabilitiesRequest requests global controller-backed feature availability.






<a name="composia-controller-v1-GetCapabilitiesResponse"></a>

### GetCapabilitiesResponse
GetCapabilitiesResponse returns global feature availability for the web UI.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| global | [GlobalCapabilities](#composia-controller-v1-GlobalCapabilities) |  |  |






<a name="composia-controller-v1-GetCurrentConfigRequest"></a>

### GetCurrentConfigRequest
GetCurrentConfigRequest requests the active redacted controller config.






<a name="composia-controller-v1-GetCurrentConfigResponse"></a>

### GetCurrentConfigResponse
GetCurrentConfigResponse contains redacted config summaries only.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| listen_addr | [string](#string) |  | listen_addr is the controller listen address. |
| git | [GitConfigSummary](#composia-controller-v1-GitConfigSummary) |  | git is the configured Git sync summary for the controller repo. |
| nodes | [NodeConfigSummary](#composia-controller-v1-NodeConfigSummary) | repeated | nodes lists configured execution nodes. |
| access_tokens | [AccessTokenSummary](#composia-controller-v1-AccessTokenSummary) | repeated | access_tokens lists token metadata without returning secret token values. |
| dns | [DNSConfigSummary](#composia-controller-v1-DNSConfigSummary) |  | dns describes optional DNS integration configuration. |
| backup | [BackupConfigSummary](#composia-controller-v1-BackupConfigSummary) |  | backup describes optional backup integration configuration. |
| secrets | [SecretsConfigSummary](#composia-controller-v1-SecretsConfigSummary) |  | secrets describes the active secrets provider configuration. |






<a name="composia-controller-v1-GetSystemStatusRequest"></a>

### GetSystemStatusRequest
GetSystemStatusRequest requests the current controller runtime state.






<a name="composia-controller-v1-GetSystemStatusResponse"></a>

### GetSystemStatusResponse
GetSystemStatusResponse describes the current controller runtime state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  | version is the controller version string. |
| now | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | now is the controller time when the response was generated. |
| configured_node_count | [uint64](#uint64) |  | configured_node_count is the number of nodes present in config. |
| online_node_count | [uint64](#uint64) |  | online_node_count is the number of nodes with a recent heartbeat. |
| repo_dir | [string](#string) |  | repo_dir is the controller-side desired state repository path. |
| state_dir | [string](#string) |  | state_dir is the controller-side persistent state directory. |
| log_dir | [string](#string) |  | log_dir is the controller-side task log directory. |






<a name="composia-controller-v1-GitConfigSummary"></a>

### GitConfigSummary
GitConfigSummary describes controller Git sync settings without credentials.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| remote_url | [string](#string) |  | remote_url is the configured Git remote URL. |
| branch | [string](#string) |  | branch is the tracked Git branch name. |
| pull_interval | [string](#string) |  | pull_interval stores the configured pull interval duration string. |
| has_auth | [bool](#bool) |  | has_auth reports whether Git auth is configured, not the auth value itself. |
| author_name | [string](#string) |  | author_name is the Git author name for controller-created commits. |
| author_email | [string](#string) |  | author_email is the Git author email for controller-created commits. |






<a name="composia-controller-v1-GlobalCapabilities"></a>

### GlobalCapabilities
GlobalCapabilities describes controller-wide feature availability.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup | [Capability](#composia-controller-v1-Capability) |  |  |
| dns | [Capability](#composia-controller-v1-Capability) |  |  |
| secrets | [Capability](#composia-controller-v1-Capability) |  |  |
| rustic_maintenance | [Capability](#composia-controller-v1-Capability) |  |  |






<a name="composia-controller-v1-NodeConfigSummary"></a>

### NodeConfigSummary
NodeConfigSummary describes one configured node entry.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the stable node identifier used across controller and agents. |
| display_name | [string](#string) |  | display_name is the human-readable node label. |
| enabled | [bool](#bool) |  | enabled reports whether the node is eligible for controller actions. |
| public_ipv4 | [string](#string) |  | public_ipv4 is empty when no public IPv4 address is configured. |
| public_ipv6 | [string](#string) |  | public_ipv6 is empty when no public IPv6 address is configured. |






<a name="composia-controller-v1-ReloadControllerConfigRequest"></a>

### ReloadControllerConfigRequest
ReloadControllerConfigRequest requests an in-process controller config reload.






<a name="composia-controller-v1-ReloadControllerConfigResponse"></a>

### ReloadControllerConfigResponse
ReloadControllerConfigResponse confirms that the reload was accepted.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| accepted | [bool](#bool) |  | accepted is true when the current config passed reload validation and was scheduled. |






<a name="composia-controller-v1-SecretsConfigSummary"></a>

### SecretsConfigSummary
SecretsConfigSummary describes the active secrets provider setup.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [string](#string) |  | provider is the configured secrets provider name. |
| has_identity | [bool](#bool) |  | has_identity reports whether an identity source is configured. |
| has_recipient | [bool](#bool) |  | has_recipient reports whether a recipient source is configured. |





 

 

 


<a name="composia-controller-v1-SystemService"></a>

### SystemService
SystemService exposes read-only controller status and config summary data.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSystemStatus | [GetSystemStatusRequest](#composia-controller-v1-GetSystemStatusRequest) | [GetSystemStatusResponse](#composia-controller-v1-GetSystemStatusResponse) | GetSystemStatus returns the current controller status and node counts. |
| ReloadControllerConfig | [ReloadControllerConfigRequest](#composia-controller-v1-ReloadControllerConfigRequest) | [ReloadControllerConfigResponse](#composia-controller-v1-ReloadControllerConfigResponse) | ReloadControllerConfig validates and applies the controller config without replacing the process. |
| GetCurrentConfig | [GetCurrentConfigRequest](#composia-controller-v1-GetCurrentConfigRequest) | [GetCurrentConfigResponse](#composia-controller-v1-GetCurrentConfigResponse) | GetCurrentConfig returns the active controller config as a redacted summary. |
| GetCapabilities | [GetCapabilitiesRequest](#composia-controller-v1-GetCapabilitiesRequest) | [GetCapabilitiesResponse](#composia-controller-v1-GetCapabilitiesResponse) | GetCapabilities returns global feature availability for the current controller state. |

 



<a name="proto_composia_controller_v1_backup-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/backup.proto



<a name="composia-controller-v1-BackupActionCapabilities"></a>

### BackupActionCapabilities
BackupActionCapabilities describes whether backup-scoped actions may run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| restore | [Capability](#composia-controller-v1-Capability) |  |  |






<a name="composia-controller-v1-BackupSummary"></a>

### BackupSummary
BackupSummary describes one backup record in list results.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup_id | [string](#string) |  |  |
| task_id | [string](#string) |  | task_id links the backup record to the originating task. |
| service_name | [string](#string) |  |  |
| data_name | [string](#string) |  |  |
| status | [string](#string) |  | status is the latest backup status string. |
| started_at | [string](#string) |  | started_at is the backup start timestamp string. |
| finished_at | [string](#string) |  | finished_at is empty until the backup reaches a terminal state. |
| node_id | [string](#string) |  |  |






<a name="composia-controller-v1-GetBackupRequest"></a>

### GetBackupRequest
GetBackupRequest identifies one backup record by ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup_id | [string](#string) |  |  |






<a name="composia-controller-v1-GetBackupResponse"></a>

### GetBackupResponse
GetBackupResponse describes one backup record in detail.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup_id | [string](#string) |  |  |
| task_id | [string](#string) |  | task_id links the backup record to the originating task. |
| service_name | [string](#string) |  |  |
| data_name | [string](#string) |  |  |
| status | [string](#string) |  | status is the latest backup status string. |
| started_at | [string](#string) |  | started_at is the backup start timestamp string. |
| finished_at | [string](#string) |  | finished_at is empty until the backup reaches a terminal state. |
| artifact_ref | [string](#string) |  | artifact_ref identifies the produced backup artifact, when present. |
| error_summary | [string](#string) |  | error_summary contains the failure summary when the backup fails. |
| actions | [BackupActionCapabilities](#composia-controller-v1-BackupActionCapabilities) |  | actions describes whether backup-scoped actions may currently run. |
| node_id | [string](#string) |  |  |






<a name="composia-controller-v1-ListBackupsRequest"></a>

### ListBackupsRequest
ListBackupsRequest filters backup records by service, status, data name, and page.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| status | [string](#string) |  | status narrows results to one backup status string when set. |
| data_name | [string](#string) |  | data_name narrows results to one service data entry when set. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |






<a name="composia-controller-v1-ListBackupsResponse"></a>

### ListBackupsResponse
ListBackupsResponse returns one page of backup records.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backups | [BackupSummary](#composia-controller-v1-BackupSummary) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-RestoreBackupRequest"></a>

### RestoreBackupRequest
RestoreBackupRequest identifies the backup record and target node for one restore task.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup_id | [string](#string) |  |  |
| node_id | [string](#string) |  | node_id identifies the destination node for the restore task. |





 

 

 


<a name="composia-controller-v1-BackupRecordService"></a>

### BackupRecordService
BackupRecordService exposes read-only backup record queries.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListBackups | [ListBackupsRequest](#composia-controller-v1-ListBackupsRequest) | [ListBackupsResponse](#composia-controller-v1-ListBackupsResponse) | ListBackups returns backup records with filtering and pagination. |
| GetBackup | [GetBackupRequest](#composia-controller-v1-GetBackupRequest) | [GetBackupResponse](#composia-controller-v1-GetBackupResponse) | GetBackup returns one backup record by ID. |
| RestoreBackup | [RestoreBackupRequest](#composia-controller-v1-RestoreBackupRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RestoreBackup starts an async restore task from one backup record. |

 



<a name="proto_composia_controller_v1_container-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/container.proto



<a name="composia-controller-v1-GetContainerLogsRequest"></a>

### GetContainerLogsRequest
GetContainerLogsRequest fetches logs for one container.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| container_id | [string](#string) |  |  |
| tail | [string](#string) |  | tail is forwarded to the Docker log tail option. |
| timestamps | [bool](#bool) |  | timestamps includes Docker timestamps in the returned log output. |






<a name="composia-controller-v1-GetContainerLogsResponse"></a>

### GetContainerLogsResponse
GetContainerLogsResponse returns one streamed log chunk.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| content | [string](#string) |  |  |






<a name="composia-controller-v1-OpenContainerExecRequest"></a>

### OpenContainerExecRequest
OpenContainerExecRequest starts an interactive exec session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| container_id | [string](#string) |  |  |
| command | [string](#string) | repeated | command stores the exec command and arguments. |
| rows | [uint32](#uint32) |  | rows is the initial terminal row count. |
| cols | [uint32](#uint32) |  | cols is the initial terminal column count. |






<a name="composia-controller-v1-OpenContainerExecResponse"></a>

### OpenContainerExecResponse
OpenContainerExecResponse returns the session identity and websocket path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id identifies the created interactive exec session. |
| websocket_path | [string](#string) |  | websocket_path is the controller websocket path for the interactive tunnel. |






<a name="composia-controller-v1-RemoveContainerRequest"></a>

### RemoveContainerRequest
RemoveContainerRequest identifies one node-scoped container deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| container_id | [string](#string) |  |  |
| force | [bool](#bool) |  |  |
| remove_volumes | [bool](#bool) |  |  |






<a name="composia-controller-v1-RemoveImageRequest"></a>

### RemoveImageRequest
RemoveImageRequest identifies one node-scoped image deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| image_id | [string](#string) |  |  |
| force | [bool](#bool) |  |  |






<a name="composia-controller-v1-RemoveNetworkRequest"></a>

### RemoveNetworkRequest
RemoveNetworkRequest identifies one node-scoped network deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| network_id | [string](#string) |  |  |






<a name="composia-controller-v1-RemoveVolumeRequest"></a>

### RemoveVolumeRequest
RemoveVolumeRequest identifies one node-scoped volume deletion.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| volume_name | [string](#string) |  |  |






<a name="composia-controller-v1-RunContainerActionRequest"></a>

### RunContainerActionRequest
RunContainerActionRequest identifies one node-scoped container action.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  |  |
| container_id | [string](#string) |  |  |
| action | [ContainerAction](#composia-controller-v1-ContainerAction) |  |  |





 


<a name="composia-controller-v1-ContainerAction"></a>

### ContainerAction
ContainerAction identifies a container lifecycle action.

| Name | Number | Description |
| ---- | ------ | ----------- |
| CONTAINER_ACTION_UNSPECIFIED | 0 | CONTAINER_ACTION_UNSPECIFIED is invalid and should not be used. |
| CONTAINER_ACTION_START | 1 | CONTAINER_ACTION_START starts the container. |
| CONTAINER_ACTION_STOP | 2 | CONTAINER_ACTION_STOP stops the container. |
| CONTAINER_ACTION_RESTART | 3 | CONTAINER_ACTION_RESTART restarts the container. |


 

 


<a name="composia-controller-v1-ContainerService"></a>

### ContainerService
ContainerService exposes container-level control and inspection entrypoints.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| RunContainerAction | [RunContainerActionRequest](#composia-controller-v1-RunContainerActionRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RunContainerAction starts an async action for one container on one node. |
| RemoveContainer | [RemoveContainerRequest](#composia-controller-v1-RemoveContainerRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RemoveContainer starts an async deletion for one container on one node. |
| GetContainerLogs | [GetContainerLogsRequest](#composia-controller-v1-GetContainerLogsRequest) | [GetContainerLogsResponse](#composia-controller-v1-GetContainerLogsResponse) stream | GetContainerLogs streams log text for one container. |
| OpenContainerExec | [OpenContainerExecRequest](#composia-controller-v1-OpenContainerExecRequest) | [OpenContainerExecResponse](#composia-controller-v1-OpenContainerExecResponse) | OpenContainerExec opens an interactive exec session for one container. |
| RemoveNetwork | [RemoveNetworkRequest](#composia-controller-v1-RemoveNetworkRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RemoveNetwork starts an async deletion for one network on one node. |
| RemoveVolume | [RemoveVolumeRequest](#composia-controller-v1-RemoveVolumeRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RemoveVolume starts an async deletion for one volume on one node. |
| RemoveImage | [RemoveImageRequest](#composia-controller-v1-RemoveImageRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RemoveImage starts an async deletion for one image on one node. |

 



<a name="proto_composia_controller_v1_node-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/node.proto



<a name="composia-controller-v1-ContainerInfo"></a>

### ContainerInfo
ContainerInfo describes one container for node-scoped list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | name is the runtime container name. |
| image | [string](#string) |  |  |
| state | [string](#string) |  | state is the low-level Docker state value. |
| status | [string](#string) |  | status is the Docker status string intended for display. |
| created | [string](#string) |  | created is the container creation timestamp string. |
| labels | [ContainerInfo.LabelsEntry](#composia-controller-v1-ContainerInfo-LabelsEntry) | repeated | labels is the container label map. |
| ports | [string](#string) | repeated | ports contains display-formatted published port mappings. |
| networks | [string](#string) | repeated | networks lists connected Docker network names. |
| image_id | [string](#string) |  | image_id is the resolved image ID. |






<a name="composia-controller-v1-ContainerInfo-LabelsEntry"></a>

### ContainerInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="composia-controller-v1-DockerStats"></a>

### DockerStats
DockerStats describes the latest Docker usage snapshot for one node.


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






<a name="composia-controller-v1-ForgetNodeRusticRequest"></a>

### ForgetNodeRusticRequest
ForgetNodeRusticRequest forgets backup snapshots for one node and dataset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| service_name | [string](#string) |  | service_name narrows the operation to one service. |
| data_name | [string](#string) |  | data_name narrows the operation to one service data entry. |






<a name="composia-controller-v1-ForgetNodeRusticResponse"></a>

### ForgetNodeRusticResponse
ForgetNodeRusticResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-GetNodeDockerStatsRequest"></a>

### GetNodeDockerStatsRequest
GetNodeDockerStatsRequest identifies the node whose Docker stats should be returned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-controller-v1-GetNodeDockerStatsResponse"></a>

### GetNodeDockerStatsResponse
GetNodeDockerStatsResponse returns the latest known Docker stats snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stats | [DockerStats](#composia-controller-v1-DockerStats) |  |  |






<a name="composia-controller-v1-GetNodeRequest"></a>

### GetNodeRequest
GetNodeRequest identifies one node by ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-controller-v1-GetNodeResponse"></a>

### GetNodeResponse
GetNodeResponse returns one node summary.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node | [NodeSummary](#composia-controller-v1-NodeSummary) |  | node is omitted when the requested node does not exist. |






<a name="composia-controller-v1-GetNodeTasksRequest"></a>

### GetNodeTasksRequest
GetNodeTasksRequest filters tasks for one node by status and page.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| status | [string](#string) |  | status narrows results to one task status string when set. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |






<a name="composia-controller-v1-GetNodeTasksResponse"></a>

### GetNodeTasksResponse
GetNodeTasksResponse returns one page of node tasks.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tasks | [TaskSummary](#composia-controller-v1-TaskSummary) | repeated |  |
| total_count | [uint32](#uint32) |  | total_count is the total number of matches before pagination. |






<a name="composia-controller-v1-ImageInfo"></a>

### ImageInfo
ImageInfo describes one Docker image for node-scoped list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| repo_tags | [string](#string) | repeated | repo_tags lists named tags referencing the image. |
| size | [int64](#int64) |  | size is the image size in bytes. |
| created | [string](#string) |  | created is the image creation timestamp string. |
| repo_digests | [string](#string) | repeated | repo_digests lists immutable digest references for the image. |
| virtual_size | [int64](#int64) |  | virtual_size is the virtual image size in bytes. |
| architecture | [string](#string) |  | architecture is the image CPU architecture string. |
| os | [string](#string) |  | os is the image operating system string. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of containers using the image. |
| is_dangling | [bool](#bool) |  | is_dangling reports whether the image is dangling. |






<a name="composia-controller-v1-InitNodeRusticRequest"></a>

### InitNodeRusticRequest
InitNodeRusticRequest initializes the Rustic repository on one node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-controller-v1-InitNodeRusticResponse"></a>

### InitNodeRusticResponse
InitNodeRusticResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-InspectNodeContainerRequest"></a>

### InspectNodeContainerRequest
InspectNodeContainerRequest identifies one node-scoped container.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| container_id | [string](#string) |  | container_id is the runtime container ID. |






<a name="composia-controller-v1-InspectNodeContainerResponse"></a>

### InspectNodeContainerResponse
InspectNodeContainerResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-controller-v1-InspectNodeImageRequest"></a>

### InspectNodeImageRequest
InspectNodeImageRequest identifies one node-scoped image.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| image_id | [string](#string) |  | image_id is the runtime image ID. |






<a name="composia-controller-v1-InspectNodeImageResponse"></a>

### InspectNodeImageResponse
InspectNodeImageResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-controller-v1-InspectNodeNetworkRequest"></a>

### InspectNodeNetworkRequest
InspectNodeNetworkRequest identifies one node-scoped network.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| network_id | [string](#string) |  | network_id is the runtime network ID. |






<a name="composia-controller-v1-InspectNodeNetworkResponse"></a>

### InspectNodeNetworkResponse
InspectNodeNetworkResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-controller-v1-InspectNodeVolumeRequest"></a>

### InspectNodeVolumeRequest
InspectNodeVolumeRequest identifies one node-scoped volume.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| volume_name | [string](#string) |  | volume_name is the runtime volume name. |






<a name="composia-controller-v1-InspectNodeVolumeResponse"></a>

### InspectNodeVolumeResponse
InspectNodeVolumeResponse returns raw Docker inspect JSON.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| raw_json | [string](#string) |  |  |






<a name="composia-controller-v1-ListNodeContainersRequest"></a>

### ListNodeContainersRequest
ListNodeContainersRequest identifies the node whose containers should be listed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-controller-v1-ListNodeContainersResponse"></a>

### ListNodeContainersResponse
ListNodeContainersResponse returns node-scoped container summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| containers | [ContainerInfo](#composia-controller-v1-ContainerInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-ListNodeImagesRequest"></a>

### ListNodeImagesRequest
ListNodeImagesRequest identifies the node whose images should be listed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-controller-v1-ListNodeImagesResponse"></a>

### ListNodeImagesResponse
ListNodeImagesResponse returns node-scoped image summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| images | [ImageInfo](#composia-controller-v1-ImageInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-ListNodeNetworksRequest"></a>

### ListNodeNetworksRequest
ListNodeNetworksRequest identifies the node whose networks should be listed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-controller-v1-ListNodeNetworksResponse"></a>

### ListNodeNetworksResponse
ListNodeNetworksResponse returns node-scoped network summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| networks | [NetworkInfo](#composia-controller-v1-NetworkInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-ListNodeVolumesRequest"></a>

### ListNodeVolumesRequest
ListNodeVolumesRequest identifies the node whose volumes should be listed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |
| search | [string](#string) |  | search is a case-insensitive substring match across key fields. |
| sort_by | [string](#string) |  | sort_by identifies the field used to sort results. |
| sort_desc | [bool](#bool) |  | sort_desc reverses the sort order when true. |






<a name="composia-controller-v1-ListNodeVolumesResponse"></a>

### ListNodeVolumesResponse
ListNodeVolumesResponse returns node-scoped volume summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volumes | [VolumeInfo](#composia-controller-v1-VolumeInfo) | repeated |  |
| total_count | [uint32](#uint32) |  |  |






<a name="composia-controller-v1-ListNodesRequest"></a>

### ListNodesRequest
ListNodesRequest requests all configured nodes.






<a name="composia-controller-v1-ListNodesResponse"></a>

### ListNodesResponse
ListNodesResponse returns all configured nodes.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| nodes | [NodeSummary](#composia-controller-v1-NodeSummary) | repeated |  |






<a name="composia-controller-v1-NetworkInfo"></a>

### NetworkInfo
NetworkInfo describes one Docker network for node-scoped list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| name | [string](#string) |  | name is the runtime network name. |
| driver | [string](#string) |  | driver is the Docker network driver name. |
| scope | [string](#string) |  | scope is the Docker network scope. |
| internal | [bool](#bool) |  | internal reports whether the network is internal-only. |
| attachable | [bool](#bool) |  | attachable reports whether standalone containers may attach to the network. |
| created | [string](#string) |  | created is the network creation timestamp string. |
| labels | [NetworkInfo.LabelsEntry](#composia-controller-v1-NetworkInfo-LabelsEntry) | repeated | labels is the network label map. |
| subnet | [string](#string) |  | subnet is the primary IPv4 subnet, when available. |
| gateway | [string](#string) |  | gateway is the primary IPv4 gateway, when available. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of connected containers. |
| ipv6_enabled | [bool](#bool) |  | ipv6_enabled reports whether IPv6 is enabled for the network. |






<a name="composia-controller-v1-NetworkInfo-LabelsEntry"></a>

### NetworkInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="composia-controller-v1-NodeActionCapabilities"></a>

### NodeActionCapabilities
NodeActionCapabilities describes whether node-scoped actions may run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| caddy_sync | [Capability](#composia-controller-v1-Capability) |  |  |
| caddy_reload | [Capability](#composia-controller-v1-Capability) |  |  |
| rustic_maintenance | [Capability](#composia-controller-v1-Capability) |  |  |






<a name="composia-controller-v1-NodeSummary"></a>

### NodeSummary
NodeSummary describes one configured node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| display_name | [string](#string) |  | display_name is the human-readable node label. |
| enabled | [bool](#bool) |  | enabled reports whether the node is eligible for controller actions. |
| is_online | [bool](#bool) |  | is_online reports whether the node has a recent heartbeat. |
| last_heartbeat | [string](#string) |  | last_heartbeat is the last recorded heartbeat timestamp string. |
| actions | [NodeActionCapabilities](#composia-controller-v1-NodeActionCapabilities) |  | actions describes whether node-scoped actions may currently run. |






<a name="composia-controller-v1-PruneNodeDockerRequest"></a>

### PruneNodeDockerRequest
PruneNodeDockerRequest prunes one Docker resource class on a node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| target | [string](#string) |  | target identifies which Docker resource group to prune. |






<a name="composia-controller-v1-PruneNodeDockerResponse"></a>

### PruneNodeDockerResponse
PruneNodeDockerResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-PruneNodeRusticRequest"></a>

### PruneNodeRusticRequest
PruneNodeRusticRequest prunes backup data for one node and dataset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| service_name | [string](#string) |  | service_name narrows the operation to one service. |
| data_name | [string](#string) |  | data_name narrows the operation to one service data entry. |






<a name="composia-controller-v1-PruneNodeRusticResponse"></a>

### PruneNodeRusticResponse
PruneNodeRusticResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-ReloadNodeCaddyRequest"></a>

### ReloadNodeCaddyRequest
ReloadNodeCaddyRequest identifies the node whose Caddy instance should be reloaded.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |






<a name="composia-controller-v1-ReloadNodeCaddyResponse"></a>

### ReloadNodeCaddyResponse
ReloadNodeCaddyResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-SyncNodeCaddyFilesRequest"></a>

### SyncNodeCaddyFilesRequest
SyncNodeCaddyFilesRequest syncs Caddy files for one node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_id | [string](#string) |  | node_id is the stable node identifier. |
| service_name | [string](#string) |  | service_name optionally narrows the sync to one service. |
| full_rebuild | [bool](#bool) |  | full_rebuild forces a full Caddy file rebuild instead of an incremental sync. |






<a name="composia-controller-v1-SyncNodeCaddyFilesResponse"></a>

### SyncNodeCaddyFilesResponse
SyncNodeCaddyFilesResponse returns the created maintenance task ID.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| task_id | [string](#string) |  |  |






<a name="composia-controller-v1-VolumeInfo"></a>

### VolumeInfo
VolumeInfo describes one Docker volume for node-scoped list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the runtime volume name. |
| driver | [string](#string) |  | driver is the Docker volume driver name. |
| mountpoint | [string](#string) |  | mountpoint is the volume mount path on the node. |
| scope | [string](#string) |  | scope is the Docker volume scope. |
| created | [string](#string) |  | created is the volume creation timestamp string. |
| labels | [VolumeInfo.LabelsEntry](#composia-controller-v1-VolumeInfo-LabelsEntry) | repeated | labels is the volume label map. |
| size_bytes | [int64](#int64) |  | size_bytes is the reported volume size in bytes. |
| containers_count | [uint32](#uint32) |  | containers_count is the number of attached containers. |
| in_use | [bool](#bool) |  | in_use reports whether any container currently uses the volume. |






<a name="composia-controller-v1-VolumeInfo-LabelsEntry"></a>

### VolumeInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 

 

 


<a name="composia-controller-v1-DockerQueryService"></a>

### DockerQueryService
DockerQueryService exposes read-only Docker resource queries for one node.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListNodeContainers | [ListNodeContainersRequest](#composia-controller-v1-ListNodeContainersRequest) | [ListNodeContainersResponse](#composia-controller-v1-ListNodeContainersResponse) | ListNodeContainers lists containers on one node. |
| InspectNodeContainer | [InspectNodeContainerRequest](#composia-controller-v1-InspectNodeContainerRequest) | [InspectNodeContainerResponse](#composia-controller-v1-InspectNodeContainerResponse) | InspectNodeContainer returns raw Docker inspect JSON for one container. |
| ListNodeNetworks | [ListNodeNetworksRequest](#composia-controller-v1-ListNodeNetworksRequest) | [ListNodeNetworksResponse](#composia-controller-v1-ListNodeNetworksResponse) | ListNodeNetworks lists networks on one node. |
| InspectNodeNetwork | [InspectNodeNetworkRequest](#composia-controller-v1-InspectNodeNetworkRequest) | [InspectNodeNetworkResponse](#composia-controller-v1-InspectNodeNetworkResponse) | InspectNodeNetwork returns raw Docker inspect JSON for one network. |
| ListNodeVolumes | [ListNodeVolumesRequest](#composia-controller-v1-ListNodeVolumesRequest) | [ListNodeVolumesResponse](#composia-controller-v1-ListNodeVolumesResponse) | ListNodeVolumes lists volumes on one node. |
| InspectNodeVolume | [InspectNodeVolumeRequest](#composia-controller-v1-InspectNodeVolumeRequest) | [InspectNodeVolumeResponse](#composia-controller-v1-InspectNodeVolumeResponse) | InspectNodeVolume returns raw Docker inspect JSON for one volume. |
| ListNodeImages | [ListNodeImagesRequest](#composia-controller-v1-ListNodeImagesRequest) | [ListNodeImagesResponse](#composia-controller-v1-ListNodeImagesResponse) | ListNodeImages lists images on one node. |
| InspectNodeImage | [InspectNodeImageRequest](#composia-controller-v1-InspectNodeImageRequest) | [InspectNodeImageResponse](#composia-controller-v1-InspectNodeImageResponse) | InspectNodeImage returns raw Docker inspect JSON for one image. |


<a name="composia-controller-v1-NodeMaintenanceService"></a>

### NodeMaintenanceService
NodeMaintenanceService triggers async maintenance work on one node.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| SyncNodeCaddyFiles | [SyncNodeCaddyFilesRequest](#composia-controller-v1-SyncNodeCaddyFilesRequest) | [SyncNodeCaddyFilesResponse](#composia-controller-v1-SyncNodeCaddyFilesResponse) | SyncNodeCaddyFiles starts a task to sync Caddy files on one node. |
| ReloadNodeCaddy | [ReloadNodeCaddyRequest](#composia-controller-v1-ReloadNodeCaddyRequest) | [ReloadNodeCaddyResponse](#composia-controller-v1-ReloadNodeCaddyResponse) | ReloadNodeCaddy starts a task to reload Caddy on one node. |
| PruneNodeDocker | [PruneNodeDockerRequest](#composia-controller-v1-PruneNodeDockerRequest) | [PruneNodeDockerResponse](#composia-controller-v1-PruneNodeDockerResponse) | PruneNodeDocker starts a task to prune Docker resources on one node. |
| InitNodeRustic | [InitNodeRusticRequest](#composia-controller-v1-InitNodeRusticRequest) | [InitNodeRusticResponse](#composia-controller-v1-InitNodeRusticResponse) | InitNodeRustic starts a task to initialize the Rustic repository on one node. |
| ForgetNodeRustic | [ForgetNodeRusticRequest](#composia-controller-v1-ForgetNodeRusticRequest) | [ForgetNodeRusticResponse](#composia-controller-v1-ForgetNodeRusticResponse) | ForgetNodeRustic starts a task to forget Rustic snapshots for one node. |
| PruneNodeRustic | [PruneNodeRusticRequest](#composia-controller-v1-PruneNodeRusticRequest) | [PruneNodeRusticResponse](#composia-controller-v1-PruneNodeRusticResponse) | PruneNodeRustic starts a task to prune Rustic data on one node. |


<a name="composia-controller-v1-NodeQueryService"></a>

### NodeQueryService
NodeQueryService exposes read-only node and node-scoped task queries.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListNodes | [ListNodesRequest](#composia-controller-v1-ListNodesRequest) | [ListNodesResponse](#composia-controller-v1-ListNodesResponse) | ListNodes returns all configured nodes with current online state. |
| GetNode | [GetNodeRequest](#composia-controller-v1-GetNodeRequest) | [GetNodeResponse](#composia-controller-v1-GetNodeResponse) | GetNode returns one node by ID. |
| GetNodeTasks | [GetNodeTasksRequest](#composia-controller-v1-GetNodeTasksRequest) | [GetNodeTasksResponse](#composia-controller-v1-GetNodeTasksResponse) | GetNodeTasks returns tasks related to one node. |
| GetNodeDockerStats | [GetNodeDockerStatsRequest](#composia-controller-v1-GetNodeDockerStatsRequest) | [GetNodeDockerStatsResponse](#composia-controller-v1-GetNodeDockerStatsResponse) | GetNodeDockerStats returns the latest Docker stats snapshot for one node. |

 



<a name="proto_composia_controller_v1_repo-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/repo.proto



<a name="composia-controller-v1-CreateRepoDirectoryRequest"></a>

### CreateRepoDirectoryRequest
CreateRepoDirectoryRequest creates one repo-relative directory path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is the repo-relative directory path to create. |
| base_revision | [string](#string) |  | base_revision protects against writing on top of an unexpected HEAD. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-CreateRepoDirectoryResponse"></a>

### CreateRepoDirectoryResponse
CreateRepoDirectoryResponse reports the commit and sync result for the create.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the directory creation. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-DeleteRepoPathRequest"></a>

### DeleteRepoPathRequest
DeleteRepoPathRequest deletes one repo-relative path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is the repo-relative path to delete. |
| base_revision | [string](#string) |  | base_revision protects against writing on top of an unexpected HEAD. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-DeleteRepoPathResponse"></a>

### DeleteRepoPathResponse
DeleteRepoPathResponse reports the commit and sync result for the delete.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the delete. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-GetRepoFileRequest"></a>

### GetRepoFileRequest
GetRepoFileRequest addresses one repo-relative file path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  |  |






<a name="composia-controller-v1-GetRepoFileResponse"></a>

### GetRepoFileResponse
GetRepoFileResponse returns a repo file and its text content.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is the repo-relative path that was read. |
| content | [string](#string) |  | content is the file content as text. |
| size | [int64](#int64) |  | size is the file size in bytes. |






<a name="composia-controller-v1-GetRepoHeadRequest"></a>

### GetRepoHeadRequest
GetRepoHeadRequest requests the current controller repo state.






<a name="composia-controller-v1-GetRepoHeadResponse"></a>

### GetRepoHeadResponse
GetRepoHeadResponse describes the current repo state known to the controller.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| head_revision | [string](#string) |  | head_revision is the current HEAD commit ID. |
| branch | [string](#string) |  |  |
| has_remote | [bool](#bool) |  |  |
| clean_worktree | [bool](#bool) |  | clean_worktree reports whether there are uncommitted local changes. |
| sync_status | [string](#string) |  | sync_status is the controller&#39;s repo sync status string. |
| last_sync_error | [string](#string) |  | last_sync_error contains the most recent sync failure, when present. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-ListRepoCommitsRequest"></a>

### ListRepoCommitsRequest
ListRepoCommitsRequest pages commit history with an opaque cursor.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| cursor | [string](#string) |  | cursor is an opaque pagination cursor from a previous response. |






<a name="composia-controller-v1-ListRepoCommitsResponse"></a>

### ListRepoCommitsResponse
ListRepoCommitsResponse returns one page of commits.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commits | [RepoCommitSummary](#composia-controller-v1-RepoCommitSummary) | repeated |  |
| next_cursor | [string](#string) |  | next_cursor is empty when there are no more results. |






<a name="composia-controller-v1-ListRepoFilesRequest"></a>

### ListRepoFilesRequest
ListRepoFilesRequest lists repo entries under path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is repo-relative. An empty path refers to the repo root. |
| recursive | [bool](#bool) |  | recursive includes all descendants under path when true. |






<a name="composia-controller-v1-ListRepoFilesResponse"></a>

### ListRepoFilesResponse
ListRepoFilesResponse returns repo entries for the requested path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [RepoFileEntry](#composia-controller-v1-RepoFileEntry) | repeated |  |






<a name="composia-controller-v1-MoveRepoPathRequest"></a>

### MoveRepoPathRequest
MoveRepoPathRequest moves one repo-relative path to another.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source_path | [string](#string) |  | source_path is the existing repo-relative path to move. |
| destination_path | [string](#string) |  | destination_path is the new repo-relative path. |
| base_revision | [string](#string) |  | base_revision protects against writing on top of an unexpected HEAD. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-MoveRepoPathResponse"></a>

### MoveRepoPathResponse
MoveRepoPathResponse reports the commit and sync result for the move.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the move. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-RepoCommitSummary"></a>

### RepoCommitSummary
RepoCommitSummary describes one commit in repo history.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  |  |
| subject | [string](#string) |  | subject is the commit subject line. |
| committed_at | [string](#string) |  | committed_at is the commit timestamp string. |






<a name="composia-controller-v1-RepoFileEntry"></a>

### RepoFileEntry
RepoFileEntry describes one direct child entry in the repo tree.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is a repo-relative path. |
| name | [string](#string) |  | name is the final path segment. |
| is_dir | [bool](#bool) |  | is_dir reports whether the entry is a directory. |
| size | [int64](#int64) |  | size is the file size in bytes. Directories may report zero. |






<a name="composia-controller-v1-RepoValidationError"></a>

### RepoValidationError
RepoValidationError identifies one validation problem in repo content.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is the repo-relative path where validation failed. |
| line | [uint32](#uint32) |  | line is the 1-based line number when the error maps to one line. |
| message | [string](#string) |  | message is a human-readable validation failure description. |






<a name="composia-controller-v1-SyncRepoRequest"></a>

### SyncRepoRequest
SyncRepoRequest asks the controller to refresh repo state from its remote.






<a name="composia-controller-v1-SyncRepoResponse"></a>

### SyncRepoResponse
SyncRepoResponse returns the repo state after a sync attempt.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| head_revision | [string](#string) |  | head_revision is the current HEAD commit ID after the sync attempt. |
| branch | [string](#string) |  | branch is the currently checked out branch name. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the sync attempt. |
| last_sync_error | [string](#string) |  | last_sync_error contains the most recent sync failure, when present. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-UpdateRepoFileRequest"></a>

### UpdateRepoFileRequest
UpdateRepoFileRequest writes one file at a repo-relative path.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | path is the repo-relative path to update. |
| content | [string](#string) |  | content is the full replacement file content. |
| base_revision | [string](#string) |  | base_revision protects against writing on top of an unexpected HEAD. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-UpdateRepoFileResponse"></a>

### UpdateRepoFileResponse
UpdateRepoFileResponse reports the commit and sync result for the write.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the write. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |






<a name="composia-controller-v1-ValidateRepoRequest"></a>

### ValidateRepoRequest
ValidateRepoRequest asks the controller to validate repo content.






<a name="composia-controller-v1-ValidateRepoResponse"></a>

### ValidateRepoResponse
ValidateRepoResponse returns all repo validation problems found.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| errors | [RepoValidationError](#composia-controller-v1-RepoValidationError) | repeated |  |





 

 

 


<a name="composia-controller-v1-RepoCommandService"></a>

### RepoCommandService
RepoCommandService applies changes to the controller Git repo.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| UpdateRepoFile | [UpdateRepoFileRequest](#composia-controller-v1-UpdateRepoFileRequest) | [UpdateRepoFileResponse](#composia-controller-v1-UpdateRepoFileResponse) | UpdateRepoFile writes one file, creates a commit, and reports sync state. |
| CreateRepoDirectory | [CreateRepoDirectoryRequest](#composia-controller-v1-CreateRepoDirectoryRequest) | [CreateRepoDirectoryResponse](#composia-controller-v1-CreateRepoDirectoryResponse) | CreateRepoDirectory creates one directory, creates a commit, and reports sync state. |
| MoveRepoPath | [MoveRepoPathRequest](#composia-controller-v1-MoveRepoPathRequest) | [MoveRepoPathResponse](#composia-controller-v1-MoveRepoPathResponse) | MoveRepoPath moves or renames one repo path. |
| DeleteRepoPath | [DeleteRepoPathRequest](#composia-controller-v1-DeleteRepoPathRequest) | [DeleteRepoPathResponse](#composia-controller-v1-DeleteRepoPathResponse) | DeleteRepoPath deletes one repo path. |
| SyncRepo | [SyncRepoRequest](#composia-controller-v1-SyncRepoRequest) | [SyncRepoResponse](#composia-controller-v1-SyncRepoResponse) | SyncRepo pulls or syncs the repo and returns the resulting state snapshot. |


<a name="composia-controller-v1-RepoQueryService"></a>

### RepoQueryService
RepoQueryService exposes read-only views of the controller Git repo.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetRepoHead | [GetRepoHeadRequest](#composia-controller-v1-GetRepoHeadRequest) | [GetRepoHeadResponse](#composia-controller-v1-GetRepoHeadResponse) | GetRepoHead returns current HEAD and sync metadata. |
| ListRepoFiles | [ListRepoFilesRequest](#composia-controller-v1-ListRepoFilesRequest) | [ListRepoFilesResponse](#composia-controller-v1-ListRepoFilesResponse) | ListRepoFiles lists direct children for one repo path. |
| GetRepoFile | [GetRepoFileRequest](#composia-controller-v1-GetRepoFileRequest) | [GetRepoFileResponse](#composia-controller-v1-GetRepoFileResponse) | GetRepoFile returns the content of one repo file. |
| ListRepoCommits | [ListRepoCommitsRequest](#composia-controller-v1-ListRepoCommitsRequest) | [ListRepoCommitsResponse](#composia-controller-v1-ListRepoCommitsResponse) | ListRepoCommits returns commit history using cursor pagination. |
| ValidateRepo | [ValidateRepoRequest](#composia-controller-v1-ValidateRepoRequest) | [ValidateRepoResponse](#composia-controller-v1-ValidateRepoResponse) | ValidateRepo runs repo validation and returns structured errors. |

 



<a name="proto_composia_controller_v1_secret-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/secret.proto



<a name="composia-controller-v1-GetSecretRequest"></a>

### GetSecretRequest
GetSecretRequest identifies one decrypted secret file for a service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| file_path | [string](#string) |  |  |






<a name="composia-controller-v1-GetSecretResponse"></a>

### GetSecretResponse
GetSecretResponse returns one decrypted secret file.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| file_path | [string](#string) |  |  |
| content | [string](#string) |  |  |






<a name="composia-controller-v1-UpdateSecretRequest"></a>

### UpdateSecretRequest
UpdateSecretRequest writes one decrypted secret file for a service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| file_path | [string](#string) |  | file_path is the repo-relative secret file path for the service. |
| content | [string](#string) |  | content is the full decrypted secret file content to store. |
| base_revision | [string](#string) |  | base_revision protects against writing on top of an unexpected HEAD. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-UpdateSecretResponse"></a>

### UpdateSecretResponse
UpdateSecretResponse reports the commit and sync result for the secret update.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the secret update. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful pull timestamp string. |





 

 

 


<a name="composia-controller-v1-SecretService"></a>

### SecretService
SecretService reads and updates encrypted secret files stored in the repo.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSecret | [GetSecretRequest](#composia-controller-v1-GetSecretRequest) | [GetSecretResponse](#composia-controller-v1-GetSecretResponse) | GetSecret returns the decrypted content for one service secret file. |
| UpdateSecret | [UpdateSecretRequest](#composia-controller-v1-UpdateSecretRequest) | [UpdateSecretResponse](#composia-controller-v1-UpdateSecretResponse) | UpdateSecret writes one secret file and reports the resulting repo sync state. |

 



<a name="proto_composia_controller_v1_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## proto/composia/controller/v1/service.proto



<a name="composia-controller-v1-GetServiceBackupsRequest"></a>

### GetServiceBackupsRequest
GetServiceBackupsRequest filters service backups by status, data name, and page.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| status | [string](#string) |  | status narrows results to one backup status string when set. |
| data_name | [string](#string) |  | data_name narrows results to one service data entry when set. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |






<a name="composia-controller-v1-GetServiceBackupsResponse"></a>

### GetServiceBackupsResponse
GetServiceBackupsResponse returns one page of service backups.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backups | [BackupSummary](#composia-controller-v1-BackupSummary) | repeated |  |
| total_count | [uint32](#uint32) |  | total_count is the total number of matches before pagination. |






<a name="composia-controller-v1-GetServiceInstanceRequest"></a>

### GetServiceInstanceRequest
GetServiceInstanceRequest identifies one service instance by service and node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id identifies the node hosting the requested instance. |
| include_containers | [bool](#bool) |  | include_containers includes instance container details when true. |






<a name="composia-controller-v1-GetServiceInstanceResponse"></a>

### GetServiceInstanceResponse
GetServiceInstanceResponse returns one service instance detail.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instance | [ServiceInstanceDetail](#composia-controller-v1-ServiceInstanceDetail) |  | instance is omitted when the requested instance does not exist. |






<a name="composia-controller-v1-GetServiceRequest"></a>

### GetServiceRequest
GetServiceRequest addresses one service by logical name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| include_containers | [bool](#bool) |  | include_containers includes per-instance container details when true. |






<a name="composia-controller-v1-GetServiceResponse"></a>

### GetServiceResponse
GetServiceResponse describes one service and all known instances.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  |  |
| runtime_status | [string](#string) |  | runtime_status is the controller&#39;s aggregated status string. |
| updated_at | [string](#string) |  | updated_at is the last known status update timestamp string. |
| nodes | [string](#string) | repeated | nodes lists the declared target nodes for this service. |
| enabled | [bool](#bool) |  | enabled reports the desired-state enabled flag for this service. |
| directory | [string](#string) |  | directory is the service directory inside the repo. |
| instances | [ServiceInstanceDetail](#composia-controller-v1-ServiceInstanceDetail) | repeated | instances lists per-node runtime details known to the controller. |
| actions | [ServiceActionCapabilities](#composia-controller-v1-ServiceActionCapabilities) |  | actions describes whether service-scoped actions may currently run. |






<a name="composia-controller-v1-GetServiceTasksRequest"></a>

### GetServiceTasksRequest
GetServiceTasksRequest filters service tasks by status and page.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| status | [string](#string) |  | status narrows results to one task status string when set. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |






<a name="composia-controller-v1-GetServiceTasksResponse"></a>

### GetServiceTasksResponse
GetServiceTasksResponse returns one page of service tasks.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tasks | [TaskSummary](#composia-controller-v1-TaskSummary) | repeated |  |
| total_count | [uint32](#uint32) |  | total_count is the total number of matches before pagination. |






<a name="composia-controller-v1-GetServiceWorkspaceRequest"></a>

### GetServiceWorkspaceRequest
GetServiceWorkspaceRequest identifies one top-level service workspace folder.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| folder | [string](#string) |  |  |






<a name="composia-controller-v1-GetServiceWorkspaceResponse"></a>

### GetServiceWorkspaceResponse
GetServiceWorkspaceResponse returns one top-level repo service workspace.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| workspace | [ServiceWorkspaceSummary](#composia-controller-v1-ServiceWorkspaceSummary) |  |  |






<a name="composia-controller-v1-ListServiceInstancesRequest"></a>

### ListServiceInstancesRequest
ListServiceInstancesRequest addresses one service by logical name.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |






<a name="composia-controller-v1-ListServiceInstancesResponse"></a>

### ListServiceInstancesResponse
ListServiceInstancesResponse returns known instances for one service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instances | [ServiceInstanceSummary](#composia-controller-v1-ServiceInstanceSummary) | repeated |  |






<a name="composia-controller-v1-ListServiceWorkspacesRequest"></a>

### ListServiceWorkspacesRequest
ListServiceWorkspacesRequest requests all top-level service workspaces.






<a name="composia-controller-v1-ListServiceWorkspacesResponse"></a>

### ListServiceWorkspacesResponse
ListServiceWorkspacesResponse returns top-level repo service workspaces.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| workspaces | [ServiceWorkspaceSummary](#composia-controller-v1-ServiceWorkspaceSummary) | repeated |  |






<a name="composia-controller-v1-ListServicesRequest"></a>

### ListServicesRequest
ListServicesRequest filters services by runtime status and page.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| runtime_status | [string](#string) |  | runtime_status narrows results to one aggregated status string when set. |
| page_size | [uint32](#uint32) |  | page_size is the requested page size. |
| page | [uint32](#uint32) |  | page is the 1-based page number. |






<a name="composia-controller-v1-ListServicesResponse"></a>

### ListServicesResponse
ListServicesResponse returns one page of service summaries.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| services | [ServiceSummary](#composia-controller-v1-ServiceSummary) | repeated |  |
| total_count | [uint32](#uint32) |  | total_count is the total number of matches before pagination. |






<a name="composia-controller-v1-MigrateServiceRequest"></a>

### MigrateServiceRequest
MigrateServiceRequest moves a service from one node to another.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| source_node_id | [string](#string) |  | source_node_id identifies the current source node. |
| target_node_id | [string](#string) |  | target_node_id identifies the destination node. |






<a name="composia-controller-v1-RunServiceActionRequest"></a>

### RunServiceActionRequest
RunServiceActionRequest starts an async action for a service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| action | [ServiceAction](#composia-controller-v1-ServiceAction) |  | action is the async operation to start. |
| node_ids | [string](#string) | repeated | node_ids optionally narrows the action to selected nodes. |
| data_names | [string](#string) | repeated | data_names narrows backup-like actions to selected data entries. |






<a name="composia-controller-v1-RunServiceInstanceActionRequest"></a>

### RunServiceInstanceActionRequest
RunServiceInstanceActionRequest starts an async action for one instance.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id identifies the node that hosts the target instance. |
| action | [ServiceInstanceAction](#composia-controller-v1-ServiceInstanceAction) |  | action is the async operation to start. |






<a name="composia-controller-v1-ServiceActionCapabilities"></a>

### ServiceActionCapabilities
ServiceActionCapabilities describes whether service-scoped actions may run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| backup | [Capability](#composia-controller-v1-Capability) |  |  |
| restore | [Capability](#composia-controller-v1-Capability) |  |  |
| migrate | [Capability](#composia-controller-v1-Capability) |  |  |
| dns_update | [Capability](#composia-controller-v1-Capability) |  |  |
| caddy_sync | [Capability](#composia-controller-v1-Capability) |  |  |






<a name="composia-controller-v1-ServiceContainerSummary"></a>

### ServiceContainerSummary
ServiceContainerSummary describes one container belonging to a service instance.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_id | [string](#string) |  | container_id is the runtime container ID. |
| name | [string](#string) |  | name is the runtime container name. |
| image | [string](#string) |  |  |
| state | [string](#string) |  | state is the low-level Docker state value. |
| status | [string](#string) |  | status is the Docker status string intended for display. |
| created | [string](#string) |  | created is the container creation timestamp string. |
| compose_project | [string](#string) |  | compose_project is the Compose project label, when present. |
| compose_service | [string](#string) |  | compose_service is the Compose service label, when present. |






<a name="composia-controller-v1-ServiceInstanceDetail"></a>

### ServiceInstanceDetail
ServiceInstanceDetail extends the instance summary with container details.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id identifies the node hosting this instance. |
| runtime_status | [string](#string) |  | runtime_status is the controller&#39;s current status string for this instance. |
| updated_at | [string](#string) |  | updated_at is the last known status update timestamp string. |
| is_declared | [bool](#bool) |  | is_declared reports whether this instance is part of desired state. |
| containers | [ServiceContainerSummary](#composia-controller-v1-ServiceContainerSummary) | repeated | containers lists runtime containers currently associated with the instance. |






<a name="composia-controller-v1-ServiceInstanceSummary"></a>

### ServiceInstanceSummary
ServiceInstanceSummary describes one service instance on one node.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  | service_name is the logical service name. |
| node_id | [string](#string) |  | node_id identifies the node hosting this instance. |
| runtime_status | [string](#string) |  | runtime_status is the controller&#39;s current status string for this instance. |
| updated_at | [string](#string) |  | updated_at is the last known status update timestamp string. |
| is_declared | [bool](#bool) |  | is_declared reports whether this instance is part of desired state. |






<a name="composia-controller-v1-ServiceSummary"></a>

### ServiceSummary
ServiceSummary describes one service for list views.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the logical service name. |
| is_declared | [bool](#bool) |  | is_declared reports whether the service exists in desired state. |
| runtime_status | [string](#string) |  | runtime_status is the controller&#39;s aggregated status string. |
| updated_at | [string](#string) |  | updated_at is the last known status update timestamp string. |
| instance_count | [uint32](#uint32) |  | instance_count is the number of known service instances. |
| running_count | [uint32](#uint32) |  | running_count is the number of running service instances. |
| target_node_count | [uint32](#uint32) |  | target_node_count is the number of declared target nodes. |






<a name="composia-controller-v1-ServiceWorkspaceSummary"></a>

### ServiceWorkspaceSummary
ServiceWorkspaceSummary describes one top-level repo workspace and any merged service state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| folder | [string](#string) |  | folder is the repo-relative top-level folder name for this workspace. |
| display_name | [string](#string) |  | display_name is the preferred workspace label for operator-facing lists. |
| service_name | [string](#string) |  | service_name is empty until the workspace has a parseable service name. |
| has_meta | [bool](#bool) |  | has_meta reports whether composia-meta.yaml exists in the workspace. |
| is_declared | [bool](#bool) |  | is_declared reports whether this workspace currently maps to a declared controller service. |
| runtime_status | [string](#string) |  | runtime_status is the merged controller status or a workspace-local placeholder. |
| updated_at | [string](#string) |  | updated_at is the last known controller update timestamp string. |
| nodes | [string](#string) | repeated | nodes lists declared target nodes when the workspace meta is parseable. |
| enabled | [bool](#bool) |  | enabled reports the desired-state enabled flag when the workspace meta is parseable. |
| actions | [ServiceActionCapabilities](#composia-controller-v1-ServiceActionCapabilities) |  | actions describes whether service-scoped actions may currently run. |






<a name="composia-controller-v1-UpdateServiceTargetNodesRequest"></a>

### UpdateServiceTargetNodesRequest
UpdateServiceTargetNodesRequest changes the full declared target node set.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_name | [string](#string) |  |  |
| node_ids | [string](#string) | repeated | node_ids replaces the full target node list for the service. |
| base_revision | [string](#string) |  | base_revision provides optimistic concurrency protection for repo writes. |
| commit_message | [string](#string) |  | commit_message is used for the generated Git commit. |






<a name="composia-controller-v1-UpdateServiceTargetNodesResponse"></a>

### UpdateServiceTargetNodesResponse
UpdateServiceTargetNodesResponse reports the resulting repo write and sync state.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| commit_id | [string](#string) |  | commit_id is the Git commit created for the desired-state change. |
| sync_status | [string](#string) |  | sync_status is the repo sync state after the write. |
| push_error | [string](#string) |  | push_error contains the last push error when sync_status is failed. |
| last_successful_pull_at | [string](#string) |  | last_successful_pull_at is the last successful controller pull timestamp string. |





 


<a name="composia-controller-v1-ServiceAction"></a>

### ServiceAction
ServiceAction identifies an async action that targets a service.

| Name | Number | Description |
| ---- | ------ | ----------- |
| SERVICE_ACTION_UNSPECIFIED | 0 | SERVICE_ACTION_UNSPECIFIED is invalid and should not be used. |
| SERVICE_ACTION_DEPLOY | 1 | SERVICE_ACTION_DEPLOY creates or reconciles the service on target nodes. |
| SERVICE_ACTION_UPDATE | 2 | SERVICE_ACTION_UPDATE refreshes the running service from desired state. |
| SERVICE_ACTION_STOP | 3 | SERVICE_ACTION_STOP stops the service. |
| SERVICE_ACTION_RESTART | 4 | SERVICE_ACTION_RESTART restarts the service. |
| SERVICE_ACTION_BACKUP | 5 | SERVICE_ACTION_BACKUP triggers a backup task. |
| SERVICE_ACTION_DNS_UPDATE | 6 | SERVICE_ACTION_DNS_UPDATE refreshes DNS records for the service. |
| SERVICE_ACTION_CADDY_SYNC | 7 | SERVICE_ACTION_CADDY_SYNC syncs related Caddy configuration. |



<a name="composia-controller-v1-ServiceInstanceAction"></a>

### ServiceInstanceAction
ServiceInstanceAction identifies an async action that targets one service instance.

| Name | Number | Description |
| ---- | ------ | ----------- |
| SERVICE_INSTANCE_ACTION_UNSPECIFIED | 0 | SERVICE_INSTANCE_ACTION_UNSPECIFIED is invalid and should not be used. |
| SERVICE_INSTANCE_ACTION_DEPLOY | 1 | SERVICE_INSTANCE_ACTION_DEPLOY creates or reconciles the instance. |
| SERVICE_INSTANCE_ACTION_UPDATE | 2 | SERVICE_INSTANCE_ACTION_UPDATE refreshes the running instance. |
| SERVICE_INSTANCE_ACTION_STOP | 3 | SERVICE_INSTANCE_ACTION_STOP stops the instance. |
| SERVICE_INSTANCE_ACTION_RESTART | 4 | SERVICE_INSTANCE_ACTION_RESTART restarts the instance. |


 

 


<a name="composia-controller-v1-ServiceCommandService"></a>

### ServiceCommandService
ServiceCommandService triggers service-level state changes and async actions.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| UpdateServiceTargetNodes | [UpdateServiceTargetNodesRequest](#composia-controller-v1-UpdateServiceTargetNodesRequest) | [UpdateServiceTargetNodesResponse](#composia-controller-v1-UpdateServiceTargetNodesResponse) | UpdateServiceTargetNodes updates the declared target nodes for a service. |
| RunServiceAction | [RunServiceActionRequest](#composia-controller-v1-RunServiceActionRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RunServiceAction starts an async action for a service and returns the task. |
| MigrateService | [MigrateServiceRequest](#composia-controller-v1-MigrateServiceRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | MigrateService starts an async service migration between two nodes. |


<a name="composia-controller-v1-ServiceInstanceService"></a>

### ServiceInstanceService
ServiceInstanceService queries and operates on one concrete service instance.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListServiceInstances | [ListServiceInstancesRequest](#composia-controller-v1-ListServiceInstancesRequest) | [ListServiceInstancesResponse](#composia-controller-v1-ListServiceInstancesResponse) | ListServiceInstances lists all instances for one service. |
| GetServiceInstance | [GetServiceInstanceRequest](#composia-controller-v1-GetServiceInstanceRequest) | [GetServiceInstanceResponse](#composia-controller-v1-GetServiceInstanceResponse) | GetServiceInstance returns the detail for one service instance on one node. |
| RunServiceInstanceAction | [RunServiceInstanceActionRequest](#composia-controller-v1-RunServiceInstanceActionRequest) | [TaskActionResponse](#composia-controller-v1-TaskActionResponse) | RunServiceInstanceAction starts an async action for one service instance. |


<a name="composia-controller-v1-ServiceQueryService"></a>

### ServiceQueryService
ServiceQueryService exposes read-only service workspace, declared service, task, and backup queries.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ListServices | [ListServicesRequest](#composia-controller-v1-ListServicesRequest) | [ListServicesResponse](#composia-controller-v1-ListServicesResponse) | ListServices returns declared services with pagination. |
| ListServiceWorkspaces | [ListServiceWorkspacesRequest](#composia-controller-v1-ListServiceWorkspacesRequest) | [ListServiceWorkspacesResponse](#composia-controller-v1-ListServiceWorkspacesResponse) | ListServiceWorkspaces returns top-level repo service workspaces with merged controller state. |
| GetService | [GetServiceRequest](#composia-controller-v1-GetServiceRequest) | [GetServiceResponse](#composia-controller-v1-GetServiceResponse) | GetService returns the full detail for a single service. |
| GetServiceWorkspace | [GetServiceWorkspaceRequest](#composia-controller-v1-GetServiceWorkspaceRequest) | [GetServiceWorkspaceResponse](#composia-controller-v1-GetServiceWorkspaceResponse) | GetServiceWorkspace returns one top-level repo service workspace. |
| GetServiceTasks | [GetServiceTasksRequest](#composia-controller-v1-GetServiceTasksRequest) | [GetServiceTasksResponse](#composia-controller-v1-GetServiceTasksResponse) | GetServiceTasks returns tasks related to one service. |
| GetServiceBackups | [GetServiceBackupsRequest](#composia-controller-v1-GetServiceBackupsRequest) | [GetServiceBackupsResponse](#composia-controller-v1-GetServiceBackupsResponse) | GetServiceBackups returns backups related to one service. |

 



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

