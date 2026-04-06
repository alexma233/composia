import type { PageServerLoad } from "./$types";

import { controllerConfig, loadBackups } from "$lib/server/controller";

export const load: PageServerLoad = async ({ url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, backups: [], totalCount: 0, page: 1 };
  }

  const page = parseInt(url.searchParams.get("page") || "1", 10) || 1;

  try {
    const result = await loadBackups(page, 20);
    return {
      ready: true,
      error: null,
      backups: result.items,
      totalCount: result.totalCount,
      page,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load backups.",
      backups: [],
      totalCount: 0,
      page: 1,
    };
  }
};
