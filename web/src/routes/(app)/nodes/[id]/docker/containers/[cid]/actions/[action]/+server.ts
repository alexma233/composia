import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  controllerConfig,
  runContainerAction,
  type ContainerAction,
} from "$lib/server/controller";
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

    return json(
      await runContainerAction(
        params.id,
        decodeURIComponent(params.cid),
        params.action,
      ),
    );
  } catch (error) {
    return jsonControllerError(error, "Failed to run container action");
  }
};

function isContainerAction(action: string): action is ContainerAction {
  return action === "start" || action === "stop" || action === "restart";
}
