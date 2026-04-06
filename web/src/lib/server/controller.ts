import { env } from "$env/dynamic/private";

type RpcRequest = Record<string, unknown>;

export type PaginatedResult<T> = {
  items: T[];
  totalCount: number;
};

export type SystemStatus = {
  version: string;
  configuredNodeCount: number;
  onlineNodeCount: number;
  controllerAddr: string;
  repoDir: string;
  stateDir: string;
  logDir: string;
};

export type ServiceSummary = {
  name: string;
  isDeclared: boolean;
  runtimeStatus: string;
  updatedAt: string;
};

export type NodeSummary = {
  nodeId: string;
  displayName: string;
  enabled: boolean;
  isOnline: boolean;
  lastHeartbeat: string;
};

export type TaskSummary = {
  taskId: string;
  type: string;
  status: string;
  serviceName: string;
  nodeId: string;
  createdAt: string;
};

export type TaskStepSummary = {
  stepName: string;
  status: string;
  startedAt: string;
  finishedAt: string;
};

export type TaskDetail = {
  taskId: string;
  type: string;
  source: string;
  serviceName: string;
  nodeId: string;
  status: string;
  createdAt: string;
  startedAt: string;
  finishedAt: string;
  repoRevision: string;
  errorSummary: string;
  logPath: string;
  triggeredBy: string;
  resultRevision: string;
  attemptOfTaskId: string;
  steps: TaskStepSummary[];
};

export type NodeDockerStats = {
  containersTotal: number;
  containersRunning: number;
  containersStopped: number;
  containersPaused: number;
  images: number;
  networks: number;
  volumes: number;
  volumesSizeBytes: number;
  disksUsageBytes: number;
  dockerServerVersion: string;
};

export type ServiceDetail = {
  name: string;
  runtimeStatus: string;
  updatedAt: string;
  node: string;
  enabled: boolean;
  directory: string;
};

export type ServiceActionResult = {
  taskId: string;
  status: string;
  repoRevision: string;
};

export type BackupSummary = {
  backupId: string;
  taskId: string;
  serviceName: string;
  dataName: string;
  status: string;
  startedAt: string;
  finishedAt: string;
};

export type RepoFileEntry = {
  path: string;
  name: string;
  isDir: boolean;
  size: number;
};

export type RepoFileContent = {
  path: string;
  content: string;
  size: number;
};

export type RepoHead = {
  branch: string;
  headRevision: string;
  cleanWorktree: boolean;
  hasRemote: boolean;
  syncStatus: string;
  lastSyncError: string;
  lastSuccessfulPullAt: string;
};

export type RepoWriteResult = {
  commitId: string;
  syncStatus: string;
  pushError: string;
  lastSuccessfulPullAt: string;
};

export type RepoSyncResult = {
  headRevision: string;
  branch: string;
  syncStatus: string;
  lastSyncError: string;
  lastSuccessfulPullAt: string;
};

export type SecretEnv = {
  serviceName: string;
  content: string;
};

export type DashboardData = {
  system: SystemStatus;
  services: ServiceSummary[];
  nodes: NodeSummary[];
  tasks: TaskSummary[];
};

export function controllerConfig() {
  const baseUrl = env.COMPOSIA_CONTROLLER_ADDR?.trim();
  const token = env.COMPOSIA_CLI_TOKEN?.trim();

  if (!baseUrl || !token) {
    return {
      ready: false as const,
      reason:
        "Set COMPOSIA_CONTROLLER_ADDR and COMPOSIA_CLI_TOKEN in the web server environment.",
    };
  }

  return {
    ready: true as const,
    baseUrl: baseUrl.replace(/\/$/, ""),
    token,
  };
}

export async function loadDashboard(): Promise<DashboardData> {
  const config = controllerConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }

  const [system, servicesResult, nodes, tasksResult] = await Promise.all([
    loadSystemStatus(),
    loadServices(8),
    loadNodes(),
    loadTasks(6),
  ]);

  return {
    system,
    services: servicesResult.items,
    nodes,
    tasks: tasksResult.items,
  };
}

export async function loadSystemStatus(): Promise<SystemStatus> {
  const config = requireControllerConfig();
  return rpcCall<SystemStatus>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.SystemService/GetSystemStatus",
    {},
  );
}

export async function loadServices(page = 1, pageSize = 50): Promise<PaginatedResult<ServiceSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    services?: ServiceSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.ServiceService/ListServices",
    { page, page_size: pageSize },
  );
  return {
    items: response.services ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function loadNodes(): Promise<NodeSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ nodes?: NodeSummary[] }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/ListNodes",
    {},
  );
  return response.nodes ?? [];
}

export async function loadTasks(page = 1, pageSize = 50): Promise<PaginatedResult<TaskSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    tasks?: TaskSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.TaskService/ListTasks",
    { page, page_size: pageSize },
  );
  return {
    items: response.tasks ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function loadBackups(page = 1, pageSize = 100): Promise<PaginatedResult<BackupSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    backups?: BackupSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.BackupRecordService/ListBackups",
    { page, page_size: pageSize },
  );
  return {
    items: response.backups ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function loadRepoHead(): Promise<RepoHead> {
  const config = requireControllerConfig();
  return rpcCall<RepoHead>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/GetRepoHead",
    {},
  );
}

export async function loadRepoEntries(path = ""): Promise<RepoFileEntry[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ entries?: RepoFileEntry[] }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/ListRepoFiles",
    { path },
  );
  return response.entries ?? [];
}

export async function loadRepoFile(path: string): Promise<RepoFileContent> {
  const config = requireControllerConfig();
  return rpcCall<RepoFileContent>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/GetRepoFile",
    { path },
  );
}

export async function updateRepoFile(
  path: string,
  content: string,
  baseRevision: string,
  commitMessage = "",
): Promise<RepoWriteResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoWriteResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/UpdateRepoFile",
    { path, content, baseRevision, commitMessage },
  );
}

export async function createRepoDirectory(
  path: string,
  baseRevision: string,
  commitMessage = "",
): Promise<RepoWriteResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoWriteResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/CreateRepoDirectory",
    { path, baseRevision, commitMessage },
  );
}

export async function moveRepoPath(
  sourcePath: string,
  destinationPath: string,
  baseRevision: string,
  commitMessage = "",
): Promise<RepoWriteResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoWriteResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/MoveRepoPath",
    { sourcePath, destinationPath, baseRevision, commitMessage },
  );
}

export async function deleteRepoPath(
  path: string,
  baseRevision: string,
  commitMessage = "",
): Promise<RepoWriteResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoWriteResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/DeleteRepoPath",
    { path, baseRevision, commitMessage },
  );
}

export async function syncRepo(): Promise<RepoSyncResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoSyncResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.RepoService/SyncRepo",
    {},
  );
}

export async function loadServiceSecret(
  serviceName: string,
): Promise<SecretEnv> {
  const config = requireControllerConfig();
  return rpcCall<SecretEnv>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.SecretService/GetServiceSecretEnv",
    { serviceName },
  );
}

export async function updateServiceSecret(
  serviceName: string,
  content: string,
  baseRevision: string,
  commitMessage = "",
): Promise<RepoWriteResult> {
  const config = requireControllerConfig();
  return rpcCall<RepoWriteResult>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.SecretService/UpdateServiceSecretEnv",
    { serviceName, content, baseRevision, commitMessage },
  );
}

export async function loadServiceDetail(
  serviceName: string,
): Promise<ServiceDetail> {
  const config = requireControllerConfig();
  return rpcCall<ServiceDetail>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.ServiceService/GetService",
    { serviceName },
  );
}

export async function loadServiceTasks(
  serviceName: string,
  page = 1,
  pageSize = 20,
): Promise<PaginatedResult<TaskSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    tasks?: TaskSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.ServiceService/GetServiceTasks",
    { service_name: serviceName, page, page_size: pageSize },
  );
  return {
    items: response.tasks ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function loadServiceBackups(
  serviceName: string,
  page = 1,
  pageSize = 20,
): Promise<PaginatedResult<BackupSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    backups?: BackupSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.ServiceService/GetServiceBackups",
    { service_name: serviceName, page, page_size: pageSize },
  );
  return {
    items: response.backups ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function deployService(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/DeployService",
    { serviceName },
  );
}

export async function updateService(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/UpdateService",
    { serviceName },
  );
}

export async function stopService(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/StopService",
    { serviceName },
  );
}

export async function restartService(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/RestartService",
    {
      serviceName,
    },
  );
}

export async function backupService(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/BackupService",
    { serviceName },
  );
}

export async function updateServiceDNS(
  serviceName: string,
): Promise<ServiceActionResult> {
  return callServiceAction(
    "/composia.controller.v1.ServiceService/UpdateServiceDNS",
    { serviceName },
  );
}

export async function loadNodeDetail(
  nodeId: string,
): Promise<NodeSummary | null> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ node?: NodeSummary }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/GetNode",
    { nodeId },
  );
  return response.node ?? null;
}

export async function loadNodeTasks(
  nodeId: string,
  page = 1,
  pageSize = 20,
): Promise<PaginatedResult<TaskSummary>> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    tasks?: TaskSummary[];
    total_count?: number;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/GetNodeTasks",
    { node_id: nodeId, page, page_size: pageSize },
  );
  return {
    items: response.tasks ?? [],
    totalCount: response.total_count ?? 0,
  };
}

export async function loadNodeDockerStats(
  nodeId: string,
): Promise<NodeDockerStats | null> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ stats?: NodeDockerStats }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/GetNodeDockerStats",
    { nodeId },
  );
  return response.stats ?? null;
}

export async function pruneNodeDocker(
  nodeId: string,
  target = "all",
): Promise<{ taskId: string }> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ task_id: string }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/PruneNodeDocker",
    { nodeId, target },
  );
  return { taskId: response.task_id };
}

export type DockerContainerSummary = {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: string;
  labels: Record<string, string>;
  ports: string[];
  networks: string[];
  imageId: string;
};

export type DockerNetworkSummary = {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
  attachable: boolean;
  created: string;
  labels: Record<string, string>;
  subnet: string;
  gateway: string;
  containersCount: number;
  ipv6Enabled: boolean;
};

export type DockerVolumeSummary = {
  name: string;
  driver: string;
  mountpoint: string;
  scope: string;
  created: string;
  labels: Record<string, string>;
  sizeBytes: number;
  containersCount: number;
  inUse: boolean;
};

export type DockerImageSummary = {
  id: string;
  repoTags: string[];
  size: number;
  created: string;
  repoDigests: string[];
  virtualSize: number;
  architecture: string;
  os: string;
  author: string;
  containersCount: number;
  isDangling: boolean;
};

function toNumber(value: unknown): number {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

export async function listNodeContainers(
  nodeId: string,
): Promise<DockerContainerSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    containers?: Array<{
      id: string;
      name: string;
      image: string;
      state: string;
      status: string;
      created: string;
      labels: Record<string, string>;
      ports: string[];
      networks: string[];
      imageId?: string;
      image_id?: string;
    }>;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/ListNodeContainers",
    { nodeId },
  );
  return (response.containers ?? []).map((c) => ({
    id: c.id,
    name: c.name,
    image: c.image,
    state: c.state,
    status: c.status,
    created: c.created,
    labels: c.labels,
    ports: c.ports ?? [],
    networks: c.networks ?? [],
    imageId: c.imageId ?? c.image_id ?? "",
  }));
}

export async function inspectNodeContainer(
  nodeId: string,
  containerId: string,
): Promise<string> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ rawJson?: string; raw_json?: string }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/InspectNodeContainer",
    { nodeId, containerId },
  );
  return response.rawJson ?? response.raw_json ?? "{}";
}

export async function listNodeNetworks(
  nodeId: string,
): Promise<DockerNetworkSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    networks?: Array<{
      id: string;
      name: string;
      driver: string;
      scope: string;
      internal: boolean;
      attachable: boolean;
      created: string;
      labels: Record<string, string>;
      subnet: string;
      gateway: string;
      containersCount?: number;
      containers_count?: number;
      ipv6Enabled?: boolean;
      ipv6_enabled?: boolean;
    }>;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/ListNodeNetworks",
    { nodeId },
  );
  return (response.networks ?? []).map((n) => ({
    id: n.id,
    name: n.name,
    driver: n.driver,
    scope: n.scope,
    internal: n.internal,
    attachable: n.attachable,
    created: n.created,
    labels: n.labels,
    subnet: n.subnet,
    gateway: n.gateway,
    containersCount: n.containersCount ?? n.containers_count ?? 0,
    ipv6Enabled: n.ipv6Enabled ?? n.ipv6_enabled ?? false,
  }));
}

export async function inspectNodeNetwork(
  nodeId: string,
  networkId: string,
): Promise<string> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ rawJson?: string; raw_json?: string }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/InspectNodeNetwork",
    { nodeId, networkId },
  );
  return response.rawJson ?? response.raw_json ?? "{}";
}

export async function listNodeVolumes(
  nodeId: string,
): Promise<DockerVolumeSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    volumes?: Array<{
      name: string;
      driver: string;
      mountpoint: string;
      scope: string;
      created: string;
      labels: Record<string, string>;
      sizeBytes?: number | string;
      size_bytes?: number | string;
      containersCount?: number;
      containers_count?: number;
      inUse?: boolean;
      in_use?: boolean;
    }>;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/ListNodeVolumes",
    { nodeId },
  );
  return (response.volumes ?? []).map((v) => ({
    name: v.name,
    driver: v.driver,
    mountpoint: v.mountpoint,
    scope: v.scope,
    created: v.created,
    labels: v.labels,
    sizeBytes: toNumber(v.sizeBytes ?? v.size_bytes),
    containersCount: v.containersCount ?? v.containers_count ?? 0,
    inUse: v.inUse ?? v.in_use ?? false,
  }));
}

export async function inspectNodeVolume(
  nodeId: string,
  volumeName: string,
): Promise<string> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ rawJson?: string; raw_json?: string }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/InspectNodeVolume",
    { nodeId, volumeName },
  );
  return response.rawJson ?? response.raw_json ?? "{}";
}

export async function listNodeImages(
  nodeId: string,
): Promise<DockerImageSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{
    images?: Array<{
      id: string;
      repoTags?: string[];
      repo_tags?: string[];
      size: number | string;
      created: string;
      repoDigests?: string[];
      repo_digests?: string[];
      virtualSize?: number | string;
      virtual_size?: number | string;
      architecture: string;
      os: string;
      author: string;
      containersCount?: number;
      containers_count?: number;
      isDangling?: boolean;
      is_dangling?: boolean;
    }>;
  }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/ListNodeImages",
    { nodeId },
  );
  return (response.images ?? []).map((i) => ({
    id: i.id,
    repoTags: i.repoTags ?? i.repo_tags ?? [],
    size: toNumber(i.size),
    created: i.created,
    repoDigests: i.repoDigests ?? i.repo_digests ?? [],
    virtualSize: toNumber(i.virtualSize ?? i.virtual_size),
    architecture: i.architecture,
    os: i.os,
    author: i.author,
    containersCount: i.containersCount ?? i.containers_count ?? 0,
    isDangling: i.isDangling ?? i.is_dangling ?? false,
  }));
}

export async function inspectNodeImage(
  nodeId: string,
  imageId: string,
): Promise<string> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ rawJson?: string; raw_json?: string }>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.NodeService/InspectNodeImage",
    { nodeId, imageId },
  );
  return response.rawJson ?? response.raw_json ?? "{}";
}

export async function loadTaskDetail(taskId: string): Promise<TaskDetail> {
  const config = requireControllerConfig();
  return rpcCall<TaskDetail>(
    config.baseUrl,
    config.token,
    "/composia.controller.v1.TaskService/GetTask",
    { taskId },
  );
}

export async function runTaskAgain(
  taskId: string,
): Promise<ServiceActionResult> {
  return callServiceAction("/composia.controller.v1.TaskService/RunTaskAgain", {
    taskId,
  });
}

function requireControllerConfig() {
  const config = controllerConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }
  return config;
}

async function callServiceAction(
  procedure: string,
  body: RpcRequest,
): Promise<ServiceActionResult> {
  const config = requireControllerConfig();
  return rpcCall<ServiceActionResult>(
    config.baseUrl,
    config.token,
    procedure,
    body,
  );
}

async function rpcCall<T>(
  baseUrl: string,
  token: string,
  procedure: string,
  body: RpcRequest,
): Promise<T> {
  const response = await fetch(`${baseUrl}${procedure}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Connect-Protocol-Version": "1",
      "Content-Type": "application/json",
      "X-Composia-Source": "web",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(
      `Controller RPC ${procedure} failed: ${response.status} ${text}`,
    );
  }

  return (await response.json()) as T;
}
