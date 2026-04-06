import type { PageServerLoad } from "./$types";

import { controllerConfig, loadBackups } from "$lib/server/controller";

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, backups: [], totalCount: 0 };
  }

  try {
    const result = await loadBackups(100);
    return {
      ready: true,
      error: null,
      backups: result.items,
      totalCount: result.totalCount,
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load backups.",
      backups: [],
      totalCount: 0,
    };
  }
};
