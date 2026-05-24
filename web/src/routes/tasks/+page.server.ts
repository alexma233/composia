import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadNodes,
  loadServices,
  loadTasks,
} from "$lib/server/controller";
import {
  parseEnumFilterValues,
  parsePageParam,
  parseTextFilterValues,
} from "$lib/filter-query";

const taskStatuses = [
  "pending",
  "running",
  "awaiting_confirmation",
  "succeeded",
  "failed",
  "cancelled",
] as const;

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

  const page = parsePageParam(url.searchParams);
  const status = parseEnumFilterValues(
    url.searchParams.getAll("status"),
    taskStatuses,
  );
  const serviceName = parseTextFilterValues(
    url.searchParams.getAll("serviceName"),
  );
  const nodeId = parseTextFilterValues(url.searchParams.getAll("nodeId"));
  const type = parseTextFilterValues(url.searchParams.getAll("type"));
  const excludeStatus = parseEnumFilterValues(
    url.searchParams.getAll("excludeStatus"),
    taskStatuses,
  );
  const excludeServiceName = parseTextFilterValues(
    url.searchParams.getAll("excludeServiceName"),
  );
  const excludeNodeId = parseTextFilterValues(
    url.searchParams.getAll("excludeNodeId"),
  );
  const rawExcludeType = parseTextFilterValues(
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
