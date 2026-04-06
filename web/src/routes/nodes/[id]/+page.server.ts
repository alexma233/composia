import type { PageServerLoad, Actions } from "./$types";

import {
  controllerConfig,
  loadNodeDetail,
  loadNodeTasks,
  loadNodeDockerStats,
  pruneNodeDocker,
} from "$lib/server/controller";

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      node: null,
      tasks: [],
      dockerStats: null,
    };
  }

  try {
    const [node, tasks, dockerStats] = await Promise.all([
      loadNodeDetail(params.id),
      loadNodeTasks(params.id),
      loadNodeDockerStats(params.id),
    ]);

    return {
      ready: true,
      error: null,
      node,
      tasks,
      dockerStats,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to load node detail.",
      node: null,
      tasks: [],
      dockerStats: null,
    };
  }
};

export const actions: Actions = {
  prune: async ({ params, request }) => {
    const config = controllerConfig();
    if (!config.ready) {
      return { success: false, error: config.reason };
    }

    const formData = await request.formData();
    const target = formData.get("target")?.toString() ?? "all";

    try {
      const result = await pruneNodeDocker(params.id, target);
      return { success: true, taskId: result.taskId };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : "Prune failed",
      };
    }
  },
};
