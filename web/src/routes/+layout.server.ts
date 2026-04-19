import type { LayoutServerLoad } from "./$types";

import {
  controllerConfig,
  loadSystemCapabilities,
} from "$lib/server/controller";
import { loadServiceWorkspaces } from "$lib/server/service-index";

export const load: LayoutServerLoad = async ({ locals }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      capabilities: null,
      user: locals.user,
      navServices: [],
      navError: config.reason,
    };
  }

  const [navServicesResult, capabilitiesResult] = await Promise.allSettled([
    loadServiceWorkspaces(),
    loadSystemCapabilities(),
  ]);

  return {
    capabilities:
      capabilitiesResult.status === "fulfilled"
        ? capabilitiesResult.value
        : null,
    user: locals.user,
    navServices:
      navServicesResult.status === "fulfilled" ? navServicesResult.value : [],
    navError:
      navServicesResult.status === "fulfilled"
        ? null
        : navServicesResult.reason instanceof Error
          ? navServicesResult.reason.message
          : "Failed to load navigation services.",
  };
};
