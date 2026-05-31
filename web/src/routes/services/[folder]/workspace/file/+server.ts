import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  loadServiceWorkspaceFiles,
  requireWorkspace,
} from "$lib/server/service-workspace-route";
import { saveServiceWorkspaceFile } from "$lib/server/service-workspace";
import { jsonApiError } from "$lib/server/controller-route";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const PUT: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      path?: string;
      content?: string;
      baseRevision?: string;
    };

    const workspace = await requireWorkspace(params.folder);
    if (!payload.path || !payload.baseRevision) {
      return jsonApiError("PATH_REVISION_REQUIRED");
    }

    const result = await saveServiceWorkspaceFile(
      params.folder,
      normalizeServiceRelativePath(payload.path),
      payload.content ?? "",
      payload.baseRevision,
    );
    const { fileTree } = await loadServiceWorkspaceFiles(params.folder);

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
        errorCode: "SAVE_FILE_FAILED",
      },
      { status: 400 },
    );
  }
};
