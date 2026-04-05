import type { PageServerLoad } from './$types';

import { controllerConfig, loadBackups } from '$lib/server/controller';

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, backups: [] };
  }

  try {
    return {
      ready: true,
      error: null,
      backups: await loadBackups(100)
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load backups.',
      backups: []
    };
  }
};
