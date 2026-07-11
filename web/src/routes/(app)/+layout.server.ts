import type { LayoutServerLoad } from "./$types";

import {
  controllerConfig,
  loadSystemCapabilities,
} from "$lib/server/controller";

export const load: LayoutServerLoad = async ({ depends }) => {
  depends("app:capabilities");
  if (!controllerConfig().ready) {
    return { capabilities: null };
  }

  return {
    capabilities: await loadSystemCapabilities().catch(() => null),
  };
};
