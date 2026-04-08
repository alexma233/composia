import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { pruneNodeRustic } from "$lib/server/controller";

export const POST: RequestHandler = async () => {
  try {
    const result = await pruneNodeRustic();
    return json({ taskId: result.taskId });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to start rustic prune.",
      },
      { status: 500 },
    );
  }
};
