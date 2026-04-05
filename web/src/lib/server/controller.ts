import { env } from '$env/dynamic/private';

type RpcRequest = Record<string, unknown>;

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

export type ServiceDetail = {
  name: string;
  runtimeStatus: string;
  updatedAt: string;
  node: string;
  enabled: boolean;
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
        'Set COMPOSIA_CONTROLLER_ADDR and COMPOSIA_CLI_TOKEN in the web server environment.'
    };
  }

  return {
    ready: true as const,
    baseUrl: baseUrl.replace(/\/$/, ''),
    token
  };
}

export async function loadDashboard(): Promise<DashboardData> {
  const config = controllerConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }

  const [system, services, nodes, tasks] = await Promise.all([
    loadSystemStatus(),
    loadServices(8),
    loadNodes(),
    loadTasks(8)
  ]);

  return {
    system,
    services,
    nodes,
    tasks
  };
}

export async function loadSystemStatus(): Promise<SystemStatus> {
  const config = requireControllerConfig();
  return rpcCall<SystemStatus>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.SystemService/GetSystemStatus',
    {}
  );
}

export async function loadServices(pageSize = 50): Promise<ServiceSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ services?: ServiceSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.ServiceService/ListServices',
    { pageSize }
  );
  return response.services ?? [];
}

export async function loadNodes(): Promise<NodeSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ nodes?: NodeSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.NodeService/ListNodes',
    {}
  );
  return response.nodes ?? [];
}

export async function loadTasks(pageSize = 50): Promise<TaskSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ tasks?: TaskSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.TaskService/ListTasks',
    { pageSize }
  );
  return response.tasks ?? [];
}

export async function loadBackups(pageSize = 100): Promise<BackupSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ backups?: BackupSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.BackupRecordService/ListBackups',
    { pageSize }
  );
  return response.backups ?? [];
}

export async function loadRepoHead(): Promise<RepoHead> {
  const config = requireControllerConfig();
  return rpcCall<RepoHead>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.RepoService/GetRepoHead',
    {}
  );
}

export async function loadRepoEntries(path = ''): Promise<RepoFileEntry[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ entries?: RepoFileEntry[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.RepoService/ListRepoFiles',
    { path }
  );
  return response.entries ?? [];
}

export async function loadRepoFile(path: string): Promise<RepoFileContent> {
  const config = requireControllerConfig();
  return rpcCall<RepoFileContent>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.RepoService/GetRepoFile',
    { path }
  );
}

export async function updateRepoFile(path: string, content: string, baseRevision: string, commitMessage = ''): Promise<{ commitId: string }> {
  const config = requireControllerConfig();
  return rpcCall<{ commitId: string }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.RepoService/UpdateRepoFile',
    { path, content, baseRevision, commitMessage }
  );
}

export async function loadServiceSecret(serviceName: string): Promise<SecretEnv> {
  const config = requireControllerConfig();
  return rpcCall<SecretEnv>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.SecretService/GetServiceSecretEnv',
    { serviceName }
  );
}

export async function updateServiceSecret(
  serviceName: string,
  content: string,
  baseRevision: string,
  commitMessage = ''
): Promise<{ commitId: string }> {
  const config = requireControllerConfig();
  return rpcCall<{ commitId: string }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.SecretService/UpdateServiceSecretEnv',
    { serviceName, content, baseRevision, commitMessage }
  );
}

export async function loadServiceDetail(serviceName: string): Promise<ServiceDetail> {
  const config = requireControllerConfig();
  return rpcCall<ServiceDetail>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.ServiceService/GetService',
    { serviceName }
  );
}

export async function loadServiceTasks(serviceName: string, pageSize = 20): Promise<TaskSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ tasks?: TaskSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.ServiceService/GetServiceTasks',
    { serviceName, pageSize }
  );
  return response.tasks ?? [];
}

export async function loadServiceBackups(serviceName: string, pageSize = 20): Promise<BackupSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ backups?: BackupSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.ServiceService/GetServiceBackups',
    { serviceName, pageSize }
  );
  return response.backups ?? [];
}

export async function loadNodeDetail(nodeId: string): Promise<NodeSummary | null> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ node?: NodeSummary }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.NodeService/GetNode',
    { nodeId }
  );
  return response.node ?? null;
}

export async function loadNodeTasks(nodeId: string, pageSize = 20): Promise<TaskSummary[]> {
  const config = requireControllerConfig();
  const response = await rpcCall<{ tasks?: TaskSummary[] }>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.NodeService/GetNodeTasks',
    { nodeId, pageSize }
  );
  return response.tasks ?? [];
}

export async function loadTaskDetail(taskId: string): Promise<TaskDetail> {
  const config = requireControllerConfig();
  return rpcCall<TaskDetail>(
    config.baseUrl,
    config.token,
    '/composia.controller.v1.TaskService/GetTask',
    { taskId }
  );
}

function requireControllerConfig() {
  const config = controllerConfig();
  if (!config.ready) {
    throw new Error(config.reason);
  }
  return config;
}

async function rpcCall<T>(baseUrl: string, token: string, procedure: string, body: RpcRequest): Promise<T> {
  const response = await fetch(`${baseUrl}${procedure}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Connect-Protocol-Version': '1',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Controller RPC ${procedure} failed: ${response.status} ${text}`);
  }

  return (await response.json()) as T;
}
