import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-node";

import { TaskService } from "$lib/gen/proto/composia/controller/v1/task_pb";
import { controllerConfig, controllerProcedure } from "$lib/server/controller";

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }
  const upstreamController = new AbortController();
  const client = createClient(
    TaskService,
    createConnectTransport({
      baseUrl: `${config.baseUrl}${controllerProcedure("")}`,
      httpVersion: "1.1",
    }),
  );

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      const textEncoder = new TextEncoder();

      try {
        const logs = client.tailTaskLogs(
          { taskId: params.id },
          {
            signal: upstreamController.signal,
            headers: {
              ...config.headers,
              Authorization: `Bearer ${config.token}`,
              "X-Composia-Source": "web",
            },
          },
        );
        for await (const message of logs) {
          if (message.content) {
            controller.enqueue(textEncoder.encode(message.content));
          }
        }

        controller.close();
      } catch (error) {
        controller.error(
          error instanceof Error
            ? error
            : new Error("Failed to stream task logs."),
        );
      }
    },
    cancel() {
      upstreamController.abort();
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
    },
  });
};
