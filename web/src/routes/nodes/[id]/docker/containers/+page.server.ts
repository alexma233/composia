import type { PageServerLoad } from "./$types";

import {
  dockerListPageSize,
  parseDockerListPageQuery,
} from "$lib/docker-list-query";
import { controllerConfig, listNodeContainers } from "$lib/server/controller";

const containerSortFields = ["name", "state", "image", "created"] as const;
const defaultContainerSortField = "name";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  const query = parseDockerListPageQuery(
    url,
    containerSortFields,
    defaultContainerSortField,
  );

  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      containers: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }

  try {
    const result = await listNodeContainers(params.id, {
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDesc: query.sortDesc,
    });

    return {
      ready: true,
      error: null,
      nodeId: params.id,
      containers: result.items,
      totalCount: result.totalCount,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load containers.",
      nodeId: params.id,
      containers: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }
};
