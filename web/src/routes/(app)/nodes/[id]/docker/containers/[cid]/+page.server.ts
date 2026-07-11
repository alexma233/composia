import type { PageServerLoad } from "./$types";

import { controllerConfig, inspectNodeContainer } from "$lib/server/controller";

export const load: PageServerLoad = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return {
      ready: false,
      error: config.reason,
      nodeId: params.id,
      containerId: decodeURIComponent(params.cid),
      initialTab: url.searchParams.get("tab") ?? "info",
      rawJson: null,
    };
  }

  try {
    const rawJson = await inspectNodeContainer(
      params.id,
      decodeURIComponent(params.cid),
    );
    return {
      ready: true,
      error: null,
      nodeId: params.id,
      containerId: decodeURIComponent(params.cid),
      initialTab: url.searchParams.get("tab") ?? "info",
      rawJson,
    };
  } catch (error) {
    return {
      ready: true,
      error:
        error instanceof Error ? error.message : "Failed to inspect container",
      nodeId: params.id,
      containerId: decodeURIComponent(params.cid),
      initialTab: url.searchParams.get("tab") ?? "info",
      rawJson: null,
    };
  }
};
