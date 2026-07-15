import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  jsonError,
  loadServiceWorkspaceSummary,
} from "$lib/server/service-workspace-route";
import { loadServiceWorkspaceFile } from "$lib/server/service-workspace";
import { decryptedSecretResponseInit } from "$lib/server/secret-response";
import {
  isEncryptedFilePath,
  normalizeServiceRelativePath,
} from "$lib/service-workspace";

export const GET: RequestHandler = async ({ params, url }) => {
  try {
    const summary = await loadServiceWorkspaceSummary(params.folder);
    const path = url.searchParams.get("path");
    const normalizedPath = path ? normalizeServiceRelativePath(path) : "";
    const file = normalizedPath
      ? await loadServiceWorkspaceFile(params.folder, normalizedPath)
      : null;

    return json(
      { ...summary, file },
      decryptedSecretResponseInit(
        Boolean(normalizedPath && isEncryptedFilePath(normalizedPath)),
      ),
    );
  } catch (error) {
    if (error instanceof Response) {
      return error;
    }
    return jsonError(error, "Failed to load service workspace.");
  }
};
