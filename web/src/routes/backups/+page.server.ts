import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadBackups,
  loadNodes,
  loadServices,
} from "$lib/server/controller";

const backupStatuses = [
  "pending",
  "running",
  "awaiting_confirmation",
  "succeeded",
  "failed",
  "cancelled",
] as const;

function parseStatuses(values: string[]): string[] {
  return values.filter((value): value is (typeof backupStatuses)[number] =>
    backupStatuses.includes(value as (typeof backupStatuses)[number]),
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
      backups: [],
      totalCount: 0,
      page: 1,
      status: [],
      serviceName: [],
      nodeId: [],
      dataName: [],
      excludeStatus: [],
      excludeServiceName: [],
      excludeNodeId: [],
      excludeDataName: [],
    };
  }

  const page = parseInt(url.searchParams.get("page") || "1", 10) || 1;
  const status = parseStatuses(url.searchParams.getAll("status"));
  const serviceName = parseTextFilters(url.searchParams.getAll("serviceName"));
  const nodeId = parseTextFilters(url.searchParams.getAll("nodeId"));
  const dataName = parseTextFilters(url.searchParams.getAll("dataName"));
  const excludeStatus = parseStatuses(url.searchParams.getAll("excludeStatus"));
  const excludeServiceName = parseTextFilters(
    url.searchParams.getAll("excludeServiceName"),
  );
  const excludeNodeId = parseTextFilters(
    url.searchParams.getAll("excludeNodeId"),
  );
  const excludeDataName = parseTextFilters(
    url.searchParams.getAll("excludeDataName"),
  );

  try {
    const [servicesResult, nodes, result] = await Promise.all([
      loadServices(1, 200),
      loadNodes(),
      loadBackups(page, 20, {
        status: status.length ? status : undefined,
        serviceName: serviceName.length ? serviceName : undefined,
        nodeId: nodeId.length ? nodeId : undefined,
        dataName: dataName.length ? dataName : undefined,
        excludeStatus: excludeStatus.length ? excludeStatus : undefined,
        excludeServiceName: excludeServiceName.length
          ? excludeServiceName
          : undefined,
        excludeNodeId: excludeNodeId.length ? excludeNodeId : undefined,
        excludeDataName: excludeDataName.length ? excludeDataName : undefined,
      }),
    ]);
    return {
      ready: true,
      error: null,
      services: servicesResult.items,
      nodes,
      backups: result.items,
      totalCount: result.totalCount,
      page,
      status,
      serviceName,
      nodeId,
      dataName,
      excludeStatus,
      excludeServiceName,
      excludeNodeId,
      excludeDataName,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load backups.",
      services: [],
      nodes: [],
      backups: [],
      totalCount: 0,
      page: 1,
      status,
      serviceName,
      nodeId,
      dataName,
      excludeStatus,
      excludeServiceName,
      excludeNodeId,
      excludeDataName,
    };
  }
};
