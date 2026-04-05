import type { PageServerLoad } from './$types';

import { controllerConfig, loadRepoHead, loadSystemStatus } from '$lib/server/controller';

export const load: PageServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return { ready: false, error: config.reason, system: null, repoHead: null };
  }

  try {
    const [system, repoHead] = await Promise.all([loadSystemStatus(), loadRepoHead()]);
    return {
      ready: true,
      error: null,
      system,
      repoHead
    };
  } catch (error) {
    return {
      ready: true,
      error: error instanceof Error ? error.message : 'Failed to load settings.',
      system: null,
      repoHead: null
    };
  }
};
