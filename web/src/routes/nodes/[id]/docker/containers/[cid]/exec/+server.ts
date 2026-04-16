import { json } from "@sveltejs/kit";
import { env } from "$env/dynamic/private";
import type { RequestHandler } from "./$types";

import { controllerConfig, openContainerExec } from "$lib/server/controller";
import { jsonControllerError } from "$lib/server/controller-route";

export const POST: RequestHandler = async ({ params, request, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  try {
    const payload = (await request.json().catch(() => ({}))) as {
      command?: string[];
      rows?: number;
      cols?: number;
    };
    const session = await openContainerExec(
      params.id,
      decodeURIComponent(params.cid),
      payload.command ?? [],
      payload.rows ?? 24,
      payload.cols ?? 80,
      url.origin,
    );
    const browserControllerBaseUrl =
      env.COMPOSIA_BROWSER_CONTROLLER_ADDR?.trim() || config.baseUrl;
    const controllerUrl = new URL(browserControllerBaseUrl);
    const wsProtocol = controllerUrl.protocol === "https:" ? "wss:" : "ws:";
    const websocketUrl = `${wsProtocol}//${controllerUrl.host}${session.websocketPath}`;
    return json({ ...session, websocketUrl });
  } catch (error) {
    return jsonControllerError(error, "Failed to open terminal");
  }
};
