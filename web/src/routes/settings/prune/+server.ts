import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { pruneNodeRustic } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const result = await pruneNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return jsonControllerError(error, "Failed to start rustic prune.");
  }
};
