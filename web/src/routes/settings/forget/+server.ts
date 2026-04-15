import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { forgetNodeRustic } from "$lib/server/controller";

export const POST: RequestHandler = async () => {
  try {
    const result = await forgetNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to start rustic forget.",
      },
      { status: 500 },
    );
  }
};
