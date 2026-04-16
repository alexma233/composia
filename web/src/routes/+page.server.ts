import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadDashboard,
  loadTasks,
} from "$lib/server/controller";

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      dashboard: null,
      totalTaskCount: 0,
    };
  }

  try {
    const [dashboard, tasksResult] = await Promise.all([
      loadDashboard(),
      loadTasks(1, 1),
    ]);

    return {
      ready: true,
      error: null,
      dashboard,
      totalTaskCount: tasksResult.totalCount,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error
          ? error.message
          : "Failed to load dashboard data.",
      dashboard: null,
      totalTaskCount: 0,
    };
  }
};
