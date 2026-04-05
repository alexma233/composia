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
