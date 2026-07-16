import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { svelteKitRouteParam } from "$lib/server/docker-route";

import { controllerConfig, removeNodeContainer } from "$lib/server/controller";
import { serializeContainerAction } from "$lib/server/container-action-queue";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    const payload = await request.json().catch(() => ({}));
    const containerId = svelteKitRouteParam(params.cid);
    return json(
      await serializeContainerAction(`${params.id}:${containerId}`, () =>
        removeNodeContainer(params.id, containerId, {
          force: payload.force === true,
          removeVolumes: payload.removeVolumes === true,
        }),
      ),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to remove container");
  }
};
