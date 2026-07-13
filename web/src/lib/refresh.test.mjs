import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { startPolling } from "./refresh.ts";

Deno.test("polling skips work while the document is hidden", async () => {
  const documentDescriptor = Object.getOwnPropertyDescriptor(
    globalThis,
    "document",
  );
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: { hidden: true },
  });

  let ticks = 0;
  const stop = startPolling(
    () => {
      ticks += 1;
    },
    { intervalMs: 5, runImmediately: true },
  );

  await new Promise((resolve) => setTimeout(resolve, 20));
  stop();

  if (documentDescriptor) {
    Object.defineProperty(globalThis, "document", documentDescriptor);
  } else {
    Reflect.deleteProperty(globalThis, "document");
  }
  assertEquals(ticks, 0);
});
