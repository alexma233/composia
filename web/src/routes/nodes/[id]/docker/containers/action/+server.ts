import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  controllerConfig,
  restartContainer,
  startContainer,
  stopContainer,
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
    switch (payload.action) {
      case "start":
        return json(await startContainer(nodeId, payload.containerId));
      case "stop":
        return json(await stopContainer(nodeId, payload.containerId));
      case "restart":
        return json(await restartContainer(nodeId, payload.containerId));
      default:
        return json({ error: `Unsupported action: ${payload.action}` }, { status: 400 });
    }
  } catch (error) {
    return json(
      { error: error instanceof Error ? error.message : "Failed to run container action" },
      { status: 500 },
    );
  }
};
