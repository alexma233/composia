import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { createMigrationRollback } from "$lib/server/controller";
import { jsonApiError, jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const body = await request.json();
    const rollbackDns = body?.rollbackDns === true;
    const deploySource = body?.deploySource === true;
    const stopTarget = body?.stopTarget === true;
    const cleanupTarget = body?.cleanupTarget === true;
    if (!rollbackDns && !deploySource && !stopTarget && !cleanupTarget) {
      return jsonApiError("ROLLBACK_ACTION_REQUIRED");
    }

    const result = await createMigrationRollback(params.id, {
      rollbackDns,
      deploySource,
      stopTarget,
      cleanupTarget,
    });
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return jsonControllerError(error, "Failed to create migration rollback.");
  }
};
