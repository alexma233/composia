import type { PageServerLoad } from "./$types";

import { controllerConfig, loadTasks } from "$lib/server/controller";

export const load: PageServerLoad = async ({ url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, tasks: [], totalCount: 0, page: 1 };
  }

  const page = parseInt(url.searchParams.get("page") || "1", 10) || 1;

  try {
    const result = await loadTasks(page, 20);
    return {
      ready: true,
      error: null,
      tasks: result.items,
      totalCount: result.totalCount,
      page,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load tasks.",
      tasks: [],
      totalCount: 0,
      page: 1,
    };
  }
};
