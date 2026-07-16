import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { svelteKitRouteParam } from "$lib/server/docker-route";

import {
  controllerConfig,
  runContainerAction,
  type ContainerAction,
} from "$lib/server/controller";
import { serializeContainerAction } from "$lib/server/container-action-queue";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    if (!isContainerAction(params.action)) {
      return json(
        { error: `Unsupported action: ${params.action}` },
        { status: 400 },
      );
    }

    const action = params.action;
    const containerId = svelteKitRouteParam(params.cid);
    return json(
      await serializeContainerAction(`${params.id}:${containerId}`, () =>
        runContainerAction(params.id, containerId, action),
      ),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to run container action");
  }
};

function isContainerAction(action: string): action is ContainerAction {
  return action === "start" || action === "stop" || action === "restart";
}
