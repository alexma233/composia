import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, loadTaskDetail } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, task: null }, { status: 503 });
  }

  try {
    return json({ task: await loadTaskDetail(params.id) });
  } catch (error) {
    return jsonControllerError(error, "Failed to load task detail.", {
      task: null,
    });
  }
};
