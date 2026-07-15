import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { serializeContainerAction } from "./container-action-queue.ts";

function deferred() {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

Deno.test("container actions run serially per container", async () => {
  const firstGate = deferred();
  const events = [];

  const first = serializeContainerAction("node-a:container-a", async () => {
    events.push("first:start");
    await firstGate.promise;
    events.push("first:end");
    return "first";
  });

  await Promise.resolve();

  const second = serializeContainerAction("node-a:container-a", async () => {
    events.push("second:start");
    return "second";
  });

  await Promise.resolve();
  assertEquals(events, ["first:start"]);

  firstGate.resolve();
  assertEquals(await first, "first");
  assertEquals(await second, "second");
  assertEquals(events, ["first:start", "first:end", "second:start"]);
});

Deno.test("container action queues are independent per container", async () => {
  const firstGate = deferred();
  const events = [];

  const first = serializeContainerAction("node-a:container-b", async () => {
    events.push("first:start");
    await firstGate.promise;
    return "first";
  });

  const second = serializeContainerAction("node-a:container-c", async () => {
    events.push("second:start");
    return "second";
  });

  await Promise.resolve();
  await Promise.resolve();
  assertEquals(events, ["first:start", "second:start"]);

  firstGate.resolve();
  assertEquals(await first, "first");
  assertEquals(await second, "second");
});
