import type { PageServerLoad } from "./$types";

import { controllerConfig, loadTaskDetail } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, task: null };
  }

  try {
    return {
      ready: true,
      error: null,
      task: await loadTaskDetail(params.id),
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load task detail.",
      task: null,
    };
  }
};
