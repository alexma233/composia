import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig, controllerProcedure } from "$lib/server/controller";

const connectProtocolVersion = "1";
const connectStreamFlagCompressed = 0x01;
const connectStreamFlagEnd = 0x02;

export const GET: RequestHandler = async ({ params, url }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  const upstreamController = new AbortController();
  let upstreamBody: ReadableStream<Uint8Array> | null = null;

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      const textEncoder = new TextEncoder();
      const textDecoder = new TextDecoder();
      let buffer = new Uint8Array(0);
      let reader: ReadableStreamDefaultReader<Uint8Array> | null = null;

      try {
        const upstream = await fetch(
          `${config.baseUrl}${controllerProcedure("/composia.controller.v1.ContainerService/GetContainerLogs")}`,
          {
            method: "POST",
            headers: {
              Authorization: `Bearer ${config.token}`,
              "Connect-Protocol-Version": connectProtocolVersion,
              "Content-Type": "application/connect+json",
              "X-Composia-Source": "web",
            },
            body: encodeConnectStreamMessage({
              nodeId: params.id,
              containerId: decodeURIComponent(params.cid),
              tail: url.searchParams.get("tail") ?? "200",
              timestamps: url.searchParams.get("timestamps") === "true",
            }),
            signal: upstreamController.signal,
          },
        );

        if (!upstream.ok || !upstream.body) {
          const text = await upstream.text();
          throw new Error(text || "Failed to stream container logs.");
        }

        upstreamBody = upstream.body;
        reader = upstream.body.getReader();

        while (true) {
          const { done, value } = await reader.read();
          if (done) {
            break;
          }
          if (!value) {
            continue;
          }
          buffer = concatUint8Arrays(buffer, value);

          while (buffer.length >= 5) {
            const flags = buffer[0] ?? 0;
            const length =
              ((buffer[1] ?? 0) << 24) |
              ((buffer[2] ?? 0) << 16) |
              ((buffer[3] ?? 0) << 8) |
              (buffer[4] ?? 0);
            if (buffer.length < 5 + length) {
              break;
            }

            const payload = buffer.slice(5, 5 + length);
            buffer = buffer.slice(5 + length);

            if ((flags & connectStreamFlagCompressed) !== 0) {
              throw new Error("Compressed Connect streams are not supported.");
            }

            const jsonPayload = JSON.parse(textDecoder.decode(payload)) as
              | { content?: string }
              | {
                  error?: { message?: string };
                };

            if ((flags & connectStreamFlagEnd) !== 0) {
              if (
                "error" in jsonPayload &&
                jsonPayload.error?.message &&
                jsonPayload.error.message.trim()
              ) {
                throw new Error(jsonPayload.error.message);
              }
              continue;
            }

            if ("content" in jsonPayload && jsonPayload.content) {
              controller.enqueue(textEncoder.encode(jsonPayload.content));
            }
          }
        }

        controller.close();
      } catch (error) {
        controller.error(
          error instanceof Error
            ? error
            : new Error("Failed to stream container logs."),
        );
      } finally {
        reader?.releaseLock();
      }
    },
    async cancel() {
      upstreamController.abort();
      await upstreamBody?.cancel();
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-store",
    },
  });
};

function encodeConnectStreamMessage(payload: Record<string, unknown>) {
  const messageBytes = new TextEncoder().encode(JSON.stringify(payload));
  const frame = new Uint8Array(5 + messageBytes.length);
  frame[0] = 0;
  frame[1] = (messageBytes.length >>> 24) & 0xff;
  frame[2] = (messageBytes.length >>> 16) & 0xff;
  frame[3] = (messageBytes.length >>> 8) & 0xff;
  frame[4] = messageBytes.length & 0xff;
  frame.set(messageBytes, 5);
  return frame;
}

function concatUint8Arrays(left: Uint8Array, right: Uint8Array) {
  const merged = new Uint8Array(left.length + right.length);
  merged.set(left, 0);
  merged.set(right, left.length);
  return merged;
}
