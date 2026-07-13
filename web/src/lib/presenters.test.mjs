import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { formatBytes, formatDuration } from "./presenters.ts";

Deno.test("formats binary byte units", () => {
  assertEquals(formatBytes(0), "0 B");
  assertEquals(formatBytes(1536), "1.5 KiB");
});

Deno.test("formats elapsed time", () => {
  const twoMinutesAgo = new Date(Date.now() - 120_000).toISOString();
  assertEquals(formatDuration(twoMinutesAgo), "2 minutes");
});
