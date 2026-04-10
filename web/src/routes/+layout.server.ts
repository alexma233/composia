import type { LayoutServerLoad } from "./$types";

import { controllerConfig } from "$lib/server/controller";
import { loadServiceWorkspaces } from "$lib/server/service-index";

export const load: LayoutServerLoad = async ({ locals }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      user: locals.user,
      navServices: [],
      navError: config.reason,
    };
  }

  try {
    return {
      user: locals.user,
      navServices: await loadServiceWorkspaces(),
      navError: null,
    };
  } catch (error) {
    return {
      user: locals.user,
      navServices: [],
      navError:
        error instanceof Error
          ? error.message
          : "Failed to load navigation services.",
    };
  }
};
