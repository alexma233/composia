import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  controllerConfig,
  runContainerAction,
  type ContainerAction,
} from "$lib/server/controller";

export const POST: RequestHandler = async ({ params, request }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  const payload = (await request.json()) as {
    action?: string;
    containerId?: string;
  };

  if (!payload.action || !payload.containerId) {
    return json({ error: "action and containerId are required" }, { status: 400 });
  }

  try {
    const nodeId = params.id;

    if (!isContainerAction(payload.action)) {
      return json({ error: `Unsupported action: ${payload.action}` }, { status: 400 });
    }

    return json(await runContainerAction(nodeId, payload.containerId, payload.action));
  } catch (error) {
    return json(
      { error: error instanceof Error ? error.message : "Failed to run container action" },
      { status: 500 },
    );
  }
};

function isContainerAction(action: string | undefined): action is ContainerAction {
  return action === "start" || action === "stop" || action === "restart";
}
