import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  loadServiceWorkspaceFiles,
  requireWorkspace,
} from "$lib/server/service-workspace-route";
import { saveServiceWorkspaceFile } from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const PUT: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      path?: string;
      content?: string;
      baseRevision?: string;
    };

    const workspace = await requireWorkspace(params.name);
    if (!payload.path || !payload.baseRevision) {
      return json(
        { error: "Path and base revision are required." },
        { status: 400 },
      );
    }

    const result = await saveServiceWorkspaceFile(
      workspace.serviceName,
      params.name,
      normalizeServiceRelativePath(payload.path),
      payload.content ?? "",
      payload.baseRevision,
    );
    const { fileTree } = await loadServiceWorkspaceFiles(params.name);

    return json({
      file: result.file,
      write: result.write,
      workspace,
      fileTree,
    });
  } catch (error) {
    return json(
      {
        error: error instanceof Error ? error.message : "Failed to save file.",
      },
      { status: 400 },
    );
  }
};
