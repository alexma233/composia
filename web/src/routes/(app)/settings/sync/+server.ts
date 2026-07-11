import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";
import { syncRepo } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async () => {
  try {
    const result = await syncRepo();
    return json({
      headRevision: result.headRevision,
      branch: result.branch,
      syncStatus: result.syncStatus,
      lastSyncError: result.lastSyncError,
      lastSuccessfulPullAt: result.lastSuccessfulPullAt,
    });
  } catch (error) {
    return jsonControllerError(error, "Failed to sync repo.");
  }
};
