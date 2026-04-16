import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadNodes,
  loadServices,
  loadTasks,
} from "$lib/server/controller";

const taskStatuses = [
  "pending",
  "running",
  "awaiting_confirmation",
  "succeeded",
  "failed",
  "cancelled",
] as const;

function parseStatuses(values: string[]): string[] {
  return values.filter((value): value is (typeof taskStatuses)[number] =>
    taskStatuses.includes(value as (typeof taskStatuses)[number]),
  );
}

function parseTextFilters(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

export const load: PageServerLoad = async ({ url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      services: [],
      nodes: [],
      tasks: [],
      totalCount: 0,
      page: 1,
      status: [],
      serviceName: [],
      nodeId: [],
      type: [],
      excludeStatus: [],
      excludeServiceName: [],
      excludeNodeId: [],
      excludeType: [],
    };
  }

  const page = parseInt(url.searchParams.get("page") || "1", 10) || 1;
  const status = parseStatuses(url.searchParams.getAll("status"));
  const serviceName = parseTextFilters(url.searchParams.getAll("serviceName"));
  const nodeId = parseTextFilters(url.searchParams.getAll("nodeId"));
  const type = parseTextFilters(url.searchParams.getAll("type"));
  const excludeStatus = parseStatuses(url.searchParams.getAll("excludeStatus"));
  const excludeServiceName = parseTextFilters(
    url.searchParams.getAll("excludeServiceName"),
  );
  const excludeNodeId = parseTextFilters(
    url.searchParams.getAll("excludeNodeId"),
  );
  const rawExcludeType = parseTextFilters(
    url.searchParams.getAll("excludeType"),
  );
  const excludeType = rawExcludeType;

  try {
    const [servicesResult, nodes, result] = await Promise.all([
      loadServices(1, 200),
      loadNodes(),
      loadTasks(page, 20, {
        status: status.length ? status : undefined,
        serviceName: serviceName.length ? serviceName : undefined,
        nodeId: nodeId.length ? nodeId : undefined,
        type: type.length ? type : undefined,
        excludeStatus: excludeStatus.length ? excludeStatus : undefined,
        excludeServiceName: excludeServiceName.length
          ? excludeServiceName
          : undefined,
        excludeNodeId: excludeNodeId.length ? excludeNodeId : undefined,
        excludeType: excludeType.length ? excludeType : undefined,
      }),
    ]);

    return {
      ready: true,
      error: null,
      services: servicesResult.items,
      nodes,
      tasks: result.items,
      totalCount: result.totalCount,
      page,
      status,
      serviceName,
      nodeId,
      type,
      excludeStatus,
      excludeServiceName,
      excludeNodeId,
      excludeType,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load tasks.",
      services: [],
      nodes: [],
      tasks: [],
      totalCount: 0,
      page: 1,
      status,
      serviceName,
      nodeId,
      type,
      excludeStatus,
      excludeServiceName,
      excludeNodeId,
      excludeType,
    };
  }
};
