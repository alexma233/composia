import type { PageServerLoad } from './$types';

import { controllerConfig, loadTasks } from '$lib/server/controller';

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, tasks: [] };
  }

  try {
    return {
      ready: true,
      error: null,
      tasks: await loadTasks(100)
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load tasks.',
      tasks: []
    };
  }
};
