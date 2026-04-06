import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";
import { runTaskAgain } from "$lib/server/controller";

export const POST: RequestHandler = async ({ params }) => {
  try {
    const result = await runTaskAgain(params.id);
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error ? error.message : "Failed to run task again.",
      },
      { status: 500 },
    );
  }
};
