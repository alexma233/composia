import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { initNodeRustic } from "$lib/server/controller";

export const POST: RequestHandler = async () => {
  try {
    const result = await initNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to start rustic init.",
      },
      { status: 500 },
    );
  }
};
