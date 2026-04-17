import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, getContainerLogs } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const GET: RequestHandler = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason, content: "" }, { status: 503 });
  }

  try {
    const content = await getContainerLogs(
      params.id,
      decodeURIComponent(params.cid),
      url.searchParams.get("tail") ?? "200",
      url.searchParams.get("timestamps") === "true",
    );
    return json({ content });
  } catch (error) {
    return jsonControllerError(error, "Failed to load container logs.", {
      content: "",
    });
  }
};
