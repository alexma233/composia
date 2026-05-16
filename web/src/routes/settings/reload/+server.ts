import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { reloadControllerConfig } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const result = await reloadControllerConfig();
    return json({ accepted: result.accepted });
  } catch (error) {
    return jsonControllerError(error, "Failed to reload controller config.");
  }
};
