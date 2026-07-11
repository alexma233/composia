import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";
import { listRepoCommits } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ url }) => {
  try {
    const pageSize = Math.min(
      Math.max(1, parseInt(url.searchParams.get("pageSize") ?? "20", 10) || 20),
      100,
    );
    const cursor = url.searchParams.get("cursor") ?? "";
    const result = await listRepoCommits(pageSize, cursor);
    return json(result);
  } catch (error) {
    return jsonControllerError(error, "Failed to load commit history.");
  }
};
