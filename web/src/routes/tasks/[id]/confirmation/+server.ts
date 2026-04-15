import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { resolveTaskConfirmation } from "$lib/server/controller";

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const body = await request.json();
    const decision = body?.decision;
    if (decision !== "approve" && decision !== "reject") {
      return json(
        { error: "decision must be approve or reject" },
        { status: 400 },
      );
    }

    const result = await resolveTaskConfirmation(params.id, decision);
    return json({
      taskId: result.taskId,
      status: result.status,
      repoRevision: result.repoRevision,
    });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to resolve task confirmation.",
      },
      { status: 500 },
    );
  }
};
