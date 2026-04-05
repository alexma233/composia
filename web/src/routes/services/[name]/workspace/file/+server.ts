import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { normalizeServiceRelativePath } from "$lib/service-workspace";
import { loadServiceWorkspace } from "$lib/server/service-index";
import {
  loadServiceWorkspaceFile,
  saveServiceWorkspaceFile,
} from "$lib/server/service-workspace";

export const GET: RequestHandler = async ({ params, url }) => {
  const path = url.searchParams.get("path") ?? "";

  try {
    const file = await loadServiceWorkspaceFile(
      params.name,
      normalizeServiceRelativePath(path),
    );
    return json({ file });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to load workspace file.",
      },
      { status: 400 },
    );
  }
};

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      path?: string;
      content?: string;
      baseRevision?: string;
    };

    if (!payload.path || !payload.baseRevision) {
      return json(
        { error: "Path and base revision are required." },
        { status: 400 },
      );
    }

    const result = await saveServiceWorkspaceFile(
      params.name,
      normalizeServiceRelativePath(payload.path),
      payload.content ?? "",
      payload.baseRevision,
    );
    const workspace = await loadServiceWorkspace(params.name);

    return json({ ...result, workspace });
  } catch (error) {
    return json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to save workspace file.",
      },
      { status: 400 },
    );
  }
};
