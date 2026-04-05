import type { PageServerLoad } from './$types';

import { controllerConfig, loadServiceBackups, loadServiceDetail, loadServiceTasks } from '$lib/server/controller';

export const load: PageServerLoad = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, service: null, tasks: [], backups: [] };
  }

  try {
    const [service, tasks, backups] = await Promise.all([
      loadServiceDetail(params.name),
      loadServiceTasks(params.name),
      loadServiceBackups(params.name)
    ]);

    return {
      ready: true,
      error: null,
      service,
      tasks,
      backups
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load service detail.',
      service: null,
      tasks: [],
      backups: []
    };
  }
};
