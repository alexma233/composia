import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { restoreBackup } from "$lib/server/controller";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const body = await request.json();
    const nodeId = body?.nodeId;
    if (typeof nodeId !== "string" || !nodeId.trim()) {
      return json({ error: "nodeId is required" }, { status: 400 });
    }

    const result = await restoreBackup(params.id, nodeId.trim());
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to start restore.",
      },
      { status: 500 },
    );
  }
};
