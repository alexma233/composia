import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { deleteRepoPath, moveRepoPath } from "$lib/server/controller";
import {
  jsonApiError,
  jsonControllerError,
} from "$lib/server/controller-route";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

export const PATCH: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as {
      folder?: string;
      baseRevision?: string;
    };
    if (!payload.baseRevision) {
      return jsonApiError("BASE_REVISION_REQUIRED");
    }

    const nextFolder = normalizeServiceRootFolder(payload.folder ?? "");
    const write = await moveRepoPath(
      params.name,
      nextFolder,
      payload.baseRevision,
    );
    return json({ write, redirectTo: `/services/${nextFolder}` });
  } catch (error) {
    return jsonControllerError(
      error,
      "Failed to rename service folder.",
      {},
      400,
    );
  }
};

export const DELETE: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as { baseRevision?: string };
    if (!payload.baseRevision) {
      return jsonApiError("BASE_REVISION_REQUIRED");
    }

    const write = await deleteRepoPath(params.name, payload.baseRevision);
    return json({ write, redirectTo: "/services" });
  } catch (error) {
    return jsonControllerError(
      error,
      "Failed to delete service folder.",
      {},
      400,
    );
  }
};

function normalizeServiceRootFolder(input: string) {
  const folder = normalizeServiceRelativePath(input);
  if (!folder || folder.includes("/")) {
    throw new Error("Service folder must stay a single top-level directory.");
  }
  return folder;
}
