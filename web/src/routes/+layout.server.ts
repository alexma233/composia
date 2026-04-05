import type { LayoutServerLoad } from './$types';

import { controllerConfig } from '$lib/server/controller';
import { loadServiceWorkspaces } from '$lib/server/service-index';

export const load: LayoutServerLoad = async () => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      navServices: [],
      navError: config.reason
    };
  }

  try {
    return {
      navServices: await loadServiceWorkspaces(),
      navError: null
    };
  } catch (error) {
    return {
      navServices: [],
      navError: error instanceof Error ? error.message : 'Failed to load navigation services.'
    };
  }
};
