import type { PageServerLoad } from './$types';

import { controllerConfig, loadDashboard } from '$lib/server/controller';

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      dashboard: null
    };
  }

  try {
    return {
      ready: true,
      error: null,
      dashboard: await loadDashboard()
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load dashboard data.',
      dashboard: null
    };
  }
};
