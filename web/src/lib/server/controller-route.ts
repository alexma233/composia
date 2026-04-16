import { json } from "@sveltejs/kit";

import {
  controllerErrorCode,
  controllerErrorMessage,
  controllerErrorStatus,
  type DockerListQuery,
} from "$lib/server/controller";

export function jsonControllerError(
  error: unknown,
  fallback: string,
  payload: Record<string, unknown> = {},
  fallbackStatus = 500,
) {
  const code = controllerErrorCode(error);
  return json(
    {
      ...payload,
      error: controllerErrorMessage(error, fallback),
      ...(code ? { code } : {}),
    },
    { status: controllerErrorStatus(error, fallbackStatus) },
  );
}

export function parseDockerListQuery(url: URL): DockerListQuery {
  return {
    page: parsePositiveInt(url.searchParams.get("page")),
    pageSize: parsePositiveInt(url.searchParams.get("pageSize")),
    search: url.searchParams.get("search")?.trim() || undefined,
    sortBy: url.searchParams.get("sortBy")?.trim() || undefined,
    sortDesc: parseBoolean(url.searchParams.get("sortDesc")),
  };
}

function parsePositiveInt(value: string | null): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

function parseBoolean(value: string | null): boolean | undefined {
  if (!value) {
    return undefined;
  }
  const normalized = value.trim().toLowerCase();
  if (normalized === "true") {
    return true;
  }
  if (normalized === "false") {
    return false;
  }
  return undefined;
}
