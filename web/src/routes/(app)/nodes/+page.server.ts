import type { PageServerLoad } from "./$types";

import { controllerConfig, loadNodes } from "$lib/server/controller";
import { isDocumentRequest } from "$lib/server/request";

export const load: PageServerLoad = async ({ depends, request }) => {
  depends("app:nodes");
  const config = controllerConfig();
  if (!config.ready) {
    return {
      content: { ready: false, error: config.reason, nodes: [] },
    };
  }

  const content = loadNodes()
    .then((nodes) => ({
      ready: true,
      error: null,
      nodes,
    }))
    .catch((error: unknown) => ({
      ready: true,
      error: error instanceof Error ? error.message : "Failed to load nodes.",
      nodes: [],
    }));

  return { content: isDocumentRequest(request) ? content : await content };
};
