import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import {
  deleteRepoPath,
  moveRepoPath,
  runServiceAction,
  type ServiceAction,
} from "$lib/server/controller";
import {
  jsonError,
  loadServiceWorkspaceSummary,
  requireDeclaredWorkspace,
  requireWorkspace,
} from "$lib/server/service-workspace-route";
import {
  createServiceWorkspaceDirectory,
  deleteServiceWorkspacePath,
  loadServiceWorkspaceFile,
  moveServiceWorkspacePath,
  saveServiceWorkspaceFile,
} from "$lib/server/service-workspace";
import { normalizeServiceRelativePath } from "$lib/service-workspace";

type WorkspacePostPayload = {
  action?: string;
  serviceAction?: string;
  path?: string;
  content?: string;
  baseRevision?: string;
  sourcePath?: string;
  destinationPath?: string;
  folder?: string;
};

type WorkspaceAction =
  | "write_file"
  | "create_directory"
  | "move"
  | "delete_path"
  | "run_service_action"
  | "rename_service_root"
  | "delete_service_root";

type WorkspaceActionHandler = (
  folder: string,
  payload: WorkspacePostPayload,
) => Promise<Response>;

const workspaceActionHandlers: Record<WorkspaceAction, WorkspaceActionHandler> = {
  write_file: handleWriteFile,
  create_directory: handleWorkspaceFs,
  move: handleWorkspaceFs,
  delete_path: handleWorkspaceFs,
  run_service_action: handleServiceAction,
  rename_service_root: handleServiceRoot,
  delete_service_root: handleServiceRoot,
};

export const GET: RequestHandler = async ({ params, url }) => {
  try {
    const summary = await loadServiceWorkspaceSummary(params.name);
    const path = url.searchParams.get("path");
    const normalizedPath = path ? normalizeServiceRelativePath(path) : "";
    const file = normalizedPath
      ? await loadServiceWorkspaceFile(
          summary.workspace.serviceName || null,
          params.name,
          normalizedPath,
        )
      : null;

    return json({ ...summary, file });
  } catch (error) {
    if (error instanceof Response) {
      return error;
    }
    return jsonError(error, "Failed to load service workspace.");
  }
};

export const POST: RequestHandler = async ({ params, request }) => {
  try {
    const payload = (await request.json()) as WorkspacePostPayload;

    if (!isWorkspaceAction(payload.action)) {
      return json({ error: "Unsupported workspace action." }, { status: 400 });
    }

    return workspaceActionHandlers[payload.action](params.name, payload);
  } catch (error) {
    if (error instanceof Response) {
      return error;
    }
    return jsonError(error, "Failed to update service workspace.");
  }
};

async function handleWriteFile(folder: string, payload: WorkspacePostPayload) {
  const workspace = await requireWorkspace(folder);
  if (!workspace.serviceName) {
    return json(
      { error: "Service is not declared. Add composia-meta.yaml before editing files." },
      { status: 400 },
    );
  }
  if (!payload.path || !payload.baseRevision) {
    return json(
      { error: "Path and base revision are required." },
      { status: 400 },
    );
  }

  const result = await saveServiceWorkspaceFile(
    workspace.serviceName,
    folder,
    normalizeServiceRelativePath(payload.path),
    payload.content ?? "",
    payload.baseRevision,
  );

  return json({ file: result.file, write: result.write });
}

async function handleWorkspaceFs(folder: string, payload: WorkspacePostPayload) {
  if (!payload.baseRevision) {
    return json({ error: "Base revision is required." }, { status: 400 });
  }

  let write;
  switch (payload.action) {
    case "create_directory":
      if (!payload.path) {
        return json({ error: "Path is required." }, { status: 400 });
      }
      write = await createServiceWorkspaceDirectory(
        folder,
        normalizeServiceRelativePath(payload.path),
        payload.baseRevision,
      );
      break;
    case "move":
      if (!payload.sourcePath || !payload.destinationPath) {
        return json(
          { error: "Source and destination paths are required." },
          { status: 400 },
        );
      }
      write = await moveServiceWorkspacePath(
        folder,
        normalizeServiceRelativePath(payload.sourcePath),
        normalizeServiceRelativePath(payload.destinationPath),
        payload.baseRevision,
      );
      break;
    case "delete_path":
      if (!payload.path) {
        return json({ error: "Path is required." }, { status: 400 });
      }
      write = await deleteServiceWorkspacePath(
        folder,
        normalizeServiceRelativePath(payload.path),
        payload.baseRevision,
      );
      break;
    default:
      return json(
        { error: "Unsupported file management action." },
        { status: 400 },
      );
  }

  const { workspace, fileTree } = await loadServiceWorkspaceSummary(folder);
  return json({ write, workspace, fileTree });
}

async function handleServiceAction(folder: string, payload: WorkspacePostPayload) {
  const workspace = await requireDeclaredWorkspace(folder);
  if (!isServiceAction(payload.serviceAction)) {
    return json({ error: "Unsupported service action." }, { status: 400 });
  }
  return json(await runServiceAction(workspace.serviceName, payload.serviceAction));
}

async function handleServiceRoot(folder: string, payload: WorkspacePostPayload) {
  if (!payload.baseRevision) {
    return json({ error: "Base revision is required." }, { status: 400 });
  }

  if (payload.action === "rename_service_root") {
    const nextFolder = normalizeServiceRootFolder(payload.folder ?? "");
    const write = await moveRepoPath(folder, nextFolder, payload.baseRevision);
    return json({ write, redirectTo: `/services/${nextFolder}` });
  }

  if (payload.action === "delete_service_root") {
    const write = await deleteRepoPath(folder, payload.baseRevision);
    return json({ write, redirectTo: "/services" });
  }

  return json({ error: "Unsupported service root action." }, { status: 400 });
}

function normalizeServiceRootFolder(input: string) {
  const folder = normalizeServiceRelativePath(input);
  if (!folder || folder.includes("/")) {
    throw new Error("Service folder must stay a single top-level directory.");
  }
  return folder;
}

function isWorkspaceAction(action: string | undefined): action is WorkspaceAction {
  if (!action) {
    return false;
  }
  return action in workspaceActionHandlers;
}

function isServiceAction(action: string | undefined): action is ServiceAction {
  return (
    action === "deploy" ||
    action === "update" ||
    action === "stop" ||
    action === "restart" ||
    action === "backup" ||
    action === "dns_update"
  );
}
