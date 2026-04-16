import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { forgetNodeRustic } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const result = await forgetNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return jsonControllerError(error, "Failed to start rustic forget.");
  }
};
