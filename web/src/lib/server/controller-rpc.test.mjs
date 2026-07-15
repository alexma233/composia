import { assert, assertEquals } from "jsr:@std/assert@1.0.19";

import {
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
