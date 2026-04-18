import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { loadBackupDetail, restoreBackup } from "$lib/server/controller";
import {
  jsonCapabilityError,
  jsonControllerError,
} from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const body = await request.json();
    const nodeId = body?.nodeId;
    if (typeof nodeId !== "string" || !nodeId.trim()) {
      return json({ error: "nodeId is required" }, { status: 400 });
    }

    const backup = await loadBackupDetail(params.id);
    if (!backup.actions.restore.enabled) {
      return jsonCapabilityError(
        backup.actions.restore.reasonCode,
        "Restore is unavailable.",
      );
    }

    const result = await restoreBackup(params.id, nodeId.trim());
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return jsonControllerError(error, "Failed to start restore.");
  }
};
