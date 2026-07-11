import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { initNodeRustic, loadSystemCapabilities } from "$lib/server/controller";
import {
  jsonCapabilityError,
  jsonControllerError,
} from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const capabilities = await loadSystemCapabilities();
    if (!capabilities.global.rusticMaintenance.enabled) {
      return jsonCapabilityError(
        capabilities.global.rusticMaintenance.reasonCode,
        "Rustic maintenance is unavailable.",
      );
    }
    const result = await initNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return jsonControllerError(error, "Failed to start rustic init.");
  }
};
