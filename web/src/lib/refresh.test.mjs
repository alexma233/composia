import { expect, test } from "bun:test";

import { startPolling } from "./refresh.ts";

test("polling skips work while the document is hidden", async () => {
  const documentDescriptor = Object.getOwnPropertyDescriptor(
    globalThis,
    "document",
  );
  Object.defineProperty(globalThis, "document", {
    configurable: true,
    value: { hidden: true },
  });

  let ticks = 0;
  const stop = startPolling(() => {
    ticks += 1;
  }, { intervalMs: 5, runImmediately: true });

  await new Promise((resolve) => setTimeout(resolve, 20));
  stop();

  if (documentDescriptor) {
    Object.defineProperty(globalThis, "document", documentDescriptor);
  } else {
    Reflect.deleteProperty(globalThis, "document");
  }
  expect(ticks).toBe(0);
});
