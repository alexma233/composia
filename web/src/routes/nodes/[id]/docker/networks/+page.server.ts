import type { PageServerLoad } from "./$types";

import {
  dockerListPageSize,
  parseDockerListPageQuery,
} from "$lib/docker-list-query";
import { controllerConfig, listNodeNetworks } from "$lib/server/controller";

const networkSortFields = ["name", "driver", "created"] as const;
const defaultNetworkSortField = "name";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  const query = parseDockerListPageQuery(
    url,
    networkSortFields,
    defaultNetworkSortField,
  );

  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      networks: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }

  try {
    const result = await listNodeNetworks(params.id, {
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
      networks: result.items,
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
        error instanceof Error ? error.message : "Failed to load networks.",
      nodeId: params.id,
      networks: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }
};
