import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  controllerErrorCode,
  controllerErrorStatus,
  loadEditableControllerConfig,
  updateEditableControllerConfig,
} from "$lib/server/controller";

export const GET: RequestHandler = async () => {
  try {
    return json(await loadEditableControllerConfig());
  } catch (error) {
    return controllerConfigError(error, "CONTROLLER_CONFIG_LOAD_FAILED");
  }
};

export const POST: RequestHandler = async ({ request, url }) => {
  const origin = request.headers.get("origin");
  if (origin !== url.origin) {
    return json(
      { errorCode: "CONTROLLER_CONFIG_INVALID_ORIGIN" },
      { status: 403 },
    );
  }
  try {
    const body = (await request.json()) as {
      yaml?: unknown;
      revision?: unknown;
    };
    if (typeof body.yaml !== "string" || typeof body.revision !== "string") {
      return json(
        { errorCode: "CONTROLLER_CONFIG_INVALID_REQUEST" },
        { status: 400 },
      );
    }
    return json(await updateEditableControllerConfig(body.yaml, body.revision));
  } catch (error) {
    return controllerConfigError(error, "CONTROLLER_CONFIG_SAVE_FAILED");
  }
};

function controllerConfigError(error: unknown, errorCode: string) {
  const code = controllerErrorCode(error);
  return json(
    { errorCode, ...(code ? { code } : {}) },
    { status: controllerErrorStatus(error) },
  );
}
