import type { LayoutServerLoad } from "./$types";

import {
  controllerConfig,
  loadSystemCapabilities,
} from "$lib/server/controller";

export const load: LayoutServerLoad = async ({ depends, locals }) => {
  depends("app:capabilities");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      capabilities: null,
      user: locals.user,
    };
  }

  const capabilities = await loadSystemCapabilities().catch(() => null);

  return {
    capabilities,
    user: locals.user,
  };
};
