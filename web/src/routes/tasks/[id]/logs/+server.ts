import { json } from "@sveltejs/kit";
import type { RequestHandler } from "./$types";

import { controllerConfig } from "$lib/server/controller";

const connectFlagCompressed = 0x01;
const connectFlagEndStream = 0x02;
const connectEnvelopeHeaderSize = 5;

function decodeEnvelopeLength(header: Uint8Array): number {
  return (
    (((header[1] ?? 0) << 24) |
      ((header[2] ?? 0) << 16) |
      ((header[3] ?? 0) << 8) |
      (header[4] ?? 0)) >>>
    0
  );
}

function encodeEnvelope(message: string): Uint8Array {
  const payload = new TextEncoder().encode(message);
  const envelope = new Uint8Array(connectEnvelopeHeaderSize + payload.length);
  envelope[0] = 0;
  envelope[1] = (payload.length >>> 24) & 0xff;
  envelope[2] = (payload.length >>> 16) & 0xff;
  envelope[3] = (payload.length >>> 8) & 0xff;
  envelope[4] = payload.length & 0xff;
  envelope.set(payload, connectEnvelopeHeaderSize);
  return envelope;
}

function envelopeBody(message: string): ArrayBuffer {
  const envelope = encodeEnvelope(message);
  const body = new Uint8Array(envelope.byteLength);
  body.set(envelope);
  return body.buffer;
}

export const GET: RequestHandler = async ({ params }) => {
  const config = controllerConfig();
  if (!config.ready) {
    return json({ error: config.reason }, { status: 503 });
  }

  const response = await fetch(
    `${config.baseUrl}/composia.controller.v1.TaskService/TailTaskLogs`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${config.token}`,
        "Connect-Protocol-Version": "1",
        "Content-Type": "application/connect+json",
        Accept: "application/connect+json",
        "X-Composia-Source": "web",
      },
      body: envelopeBody(JSON.stringify({ taskId: params.id })),
    },
  );

  if (!response.ok || !response.body) {
    const text = await response.text();
    return json(
      {
        error:
          text ||
          `Controller task log stream failed with status ${response.status}`,
      },
      { status: response.status || 500 },
    );
  }

  const upstream = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = new Uint8Array(0);

  const stream = new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        while (true) {
          const { done, value } = await upstream.read();
          if (done) {
            controller.close();
            return;
          }

          if (value && value.length > 0) {
            const merged = new Uint8Array(buffer.length + value.length);
            merged.set(buffer);
            merged.set(value, buffer.length);
            buffer = merged;
          }

          while (buffer.length >= connectEnvelopeHeaderSize) {
            const flags = buffer[0] ?? 0;
            const messageLength = decodeEnvelopeLength(buffer);
            const envelopeLength = connectEnvelopeHeaderSize + messageLength;
            if (buffer.length < envelopeLength) {
              break;
            }

            const payload = buffer.slice(
              connectEnvelopeHeaderSize,
              envelopeLength,
            );
            buffer = buffer.slice(envelopeLength);

            if ((flags & connectFlagCompressed) !== 0) {
              throw new Error("Compressed Connect streams are not supported.");
            }

            if ((flags & connectFlagEndStream) !== 0) {
              controller.close();
              return;
            }

            const chunk = decoder.decode(payload);
            if (!chunk) {
              continue;
            }

            const parsed = JSON.parse(chunk) as { content?: string };
            if (parsed.content) {
              controller.enqueue(payload);
              return;
            }
          }
        }
      } catch (error) {
        controller.error(error);
      }
    },

    async cancel() {
      await upstream.cancel();
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "no-cache",
    },
  });
};
