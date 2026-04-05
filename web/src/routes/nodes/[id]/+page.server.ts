import type { PageServerLoad } from "./$types";

import {
  controllerConfig,
  loadNodeDetail,
  loadNodeTasks,
} from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, node: null, tasks: [] };
  }

  try {
    const [node, tasks] = await Promise.all([
      loadNodeDetail(params.id),
      loadNodeTasks(params.id),
    ]);

    return {
      ready: true,
      error: null,
      node,
      tasks,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load node detail.",
      node: null,
      tasks: [],
    };
  }
};
