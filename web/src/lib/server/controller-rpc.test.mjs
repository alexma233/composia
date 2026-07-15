import { assert, assertEquals, assertRejects } from "jsr:@std/assert@1.0.19";

import {
  ControllerRpcError,
  controllerRpcCall,
  controllerRpcDeadline,
  controllerRpcHeaders,
} from "./controller-rpc.ts";

Deno.test("builds unary controller headers with a shared deadline", () => {
  assertEquals(
    controllerRpcHeaders("token", { "X-Proxy": "yes" }, { "X-Extra": "1" }),
    {
      "X-Proxy": "yes",
      Authorization: "Bearer token",
      "Connect-Protocol-Version": "1",
      "Connect-Timeout-Ms": "30000",
      "Content-Type": "application/json",
      "X-Composia-Source": "web",
      "X-Extra": "1",
    },
  );
});

Deno.test(
  "aborts unary controller RPCs when the request is cancelled",
  async () => {
    const requestController = new AbortController();
    const deadline = controllerRpcDeadline(requestController.signal, 1_000);

    requestController.abort();
    await Promise.resolve();

    assert(deadline.signal.aborted);
    assert(!deadline.timedOut());
    deadline.cancel();
  },
);

Deno.test("aborts unary controller RPCs at the shared deadline", async () => {
  const deadline = controllerRpcDeadline(undefined, 1);
  await new Promise((resolve) => setTimeout(resolve, 10));

  assert(deadline.signal.aborted);
  assert(deadline.timedOut());
  deadline.cancel();
});

Deno.test(
  "keeps unary controller RPC deadline active while parsing the response body",
  async () => {
    const originalFetch = globalThis.fetch;
    let safetyTimeout;

    globalThis.fetch = (_input, init) => {
      const signal = init?.signal;
      const body = new ReadableStream({
        start(controller) {
          signal?.addEventListener(
            "abort",
            () => controller.error(signal.reason),
            { once: true },
          );
          safetyTimeout = setTimeout(
            () => controller.error(new Error("body read was not aborted")),
            100,
          );
        },
      });
      return Promise.resolve(new Response(body));
    };

    try {
      const error = await assertRejects(
        () =>
          controllerRpcCall({
            baseUrl: "https://controller.test",
            token: "token",
            procedure: "/slow",
            body: {},
            controllerHeaders: {},
            timeoutMs: 1,
          }),
        ControllerRpcError,
      );

      assertEquals(error.status, 504);
      assertEquals(error.code, "DEADLINE_EXCEEDED");
    } finally {
      clearTimeout(safetyTimeout);
      globalThis.fetch = originalFetch;
    }
  },
);
