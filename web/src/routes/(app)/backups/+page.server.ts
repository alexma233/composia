import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadBackups,
  loadNodes,
  loadServices,
} from "$lib/server/controller";
import {
  parseEnumFilterValues,
  parsePageParam,
  parseTextFilterValues,
} from "$lib/filter-query";
import { isDocumentRequest } from "$lib/server/request";

const backupStatuses = [
  "pending",
  "running",
  "awaiting_confirmation",
  "succeeded",
  "failed",
  "cancelled",
] as const;

export const load: PageServerLoad = async ({ depends, request, url }) => {
  depends("app:backups");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      content: {
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
      },
    };
  }

  const page = parsePageParam(url.searchParams);
  const status = parseEnumFilterValues(
    url.searchParams.getAll("status"),
    backupStatuses,
  );
  const serviceName = parseTextFilterValues(
    url.searchParams.getAll("serviceName"),
  );
  const nodeId = parseTextFilterValues(url.searchParams.getAll("nodeId"));
  const dataName = parseTextFilterValues(url.searchParams.getAll("dataName"));
  const excludeStatus = parseEnumFilterValues(
    url.searchParams.getAll("excludeStatus"),
    backupStatuses,
  );
  const excludeServiceName = parseTextFilterValues(
    url.searchParams.getAll("excludeServiceName"),
  );
  const excludeNodeId = parseTextFilterValues(
    url.searchParams.getAll("excludeNodeId"),
  );
  const excludeDataName = parseTextFilterValues(
    url.searchParams.getAll("excludeDataName"),
  );

  const content = Promise.all([
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
  ])
    .then(([servicesResult, nodes, result]) => ({
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
    }))
    .catch((error: unknown) => ({
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
    }));

  return { content: isDocumentRequest(request) ? content : await content };
};
