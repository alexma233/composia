import { gunzipSync } from "node:zlib";
import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { compressResponse, prepareCompressionRequest } from "./compression.mjs";

Deno.test(
  "compressResponse negotiates gzip for compressible responses",
  async () => {
    const request = new Request("https://example.test", {
      headers: { "Accept-Encoding": "br, gzip" },
    });
    const response = compressResponse(
      request,
      new Response("content".repeat(400), {
        headers: {
          "Content-Type": "text/plain",
          ETag: '"content"',
        },
      }),
    );

    assertEquals(response.headers.get("content-encoding"), "gzip");
    assertEquals(response.headers.get("vary"), "Accept-Encoding");
    assertEquals(response.headers.get("etag"), '"content-gzip"');
    assertEquals(
      new TextDecoder().decode(
        gunzipSync(new Uint8Array(await response.arrayBuffer())),
      ),
      "content".repeat(400),
    );
  },
);

Deno.test(
  "compressResponse keeps small and streaming event responses unchanged",
  () => {
    const request = new Request("https://example.test", {
      headers: { "Accept-Encoding": "gzip" },
    });
    const small = new Response("small", {
      headers: { "Content-Type": "text/plain", "Content-Length": "5" },
    });
    const events = new Response("data: ok\n\n", {
      headers: { "Content-Type": "text/event-stream" },
    });

    assertEquals(compressResponse(request, small), small);
    assertEquals(compressResponse(request, events), events);
  },
);

Deno.test("compressResponse honors explicit gzip refusal", () => {
  const result = compressResponse(
    new Request("https://example.test", {
      headers: { "Accept-Encoding": "gzip;q=0.0, *;q=1" },
    }),
    new Response("content".repeat(400), {
      headers: { "Content-Type": "text/plain" },
    }),
  );

  assertEquals(result.headers.get("content-encoding"), null);
  assertEquals(result.headers.get("vary"), "Accept-Encoding");
});

Deno.test("compressResponse preserves HEAD semantics", () => {
  const response = compressResponse(
    new Request("https://example.test", {
      method: "HEAD",
      headers: { "Accept-Encoding": "gzip" },
    }),
    new Response(null, {
      headers: { "Content-Type": "text/html", "Content-Length": "4096" },
    }),
  );

  assertEquals(response.body, null);
  assertEquals(response.headers.get("content-encoding"), null);
  assertEquals(response.headers.get("content-length"), "4096");
  assertEquals(response.headers.get("vary"), "Accept-Encoding");
});

Deno.test("compression ETags round-trip through conditional requests", () => {
  const request = new Request("https://example.test", {
    headers: {
      "Accept-Encoding": "gzip",
      "If-None-Match": '"old", "content-gzip"',
    },
  });
  const handlerRequest = prepareCompressionRequest(request);
  assertEquals(handlerRequest.headers.get("if-none-match"), '"content"');

  const response = compressResponse(
    request,
    new Response(null, { status: 304, headers: { ETag: '"content"' } }),
  );
  assertEquals(response.status, 304);
  assertEquals(response.headers.get("etag"), '"content-gzip"');
  assertEquals(response.headers.get("content-encoding"), "gzip");
  assertEquals(response.headers.get("vary"), "Accept-Encoding");
});

Deno.test("identity validators cannot match the gzip representation", () => {
  const request = new Request("https://example.test", {
    headers: {
      "Accept-Encoding": "gzip",
      "If-None-Match": 'W/"identity"',
    },
  });
  assertEquals(
    prepareCompressionRequest(request).headers.get("if-none-match"),
    null,
  );
});
