import type { PageServerLoad } from "./$types";

import {
  dockerListPageSize,
  parseDockerListPageQuery,
} from "$lib/docker-list-query";
import { controllerConfig, listNodeImages } from "$lib/server/controller";

const imageSortFields = ["name", "size", "created"] as const;
const defaultImageSortField = "name";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  const query = parseDockerListPageQuery(
    url,
    imageSortFields,
    defaultImageSortField,
  );

  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      images: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }

  try {
    const result = await listNodeImages(params.id, {
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
      images: result.items,
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
      error: error instanceof Error ? error.message : "Failed to load images.",
      nodeId: params.id,
      images: [],
      totalCount: 0,
      page: query.page,
      pageSize: dockerListPageSize,
      search: query.search,
      sortBy: query.sortBy,
      sortDirection: query.sortDirection,
    };
  }
};
