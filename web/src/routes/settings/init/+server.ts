import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { initNodeRustic } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const result = await initNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return jsonControllerError(error, "Failed to start rustic init.");
  }
};
