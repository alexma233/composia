import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";
import { runTaskAgain } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params }) => {
  try {
    const result = await runTaskAgain(params.id);
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return jsonControllerError(error, "Failed to run task again.");
  }
};
