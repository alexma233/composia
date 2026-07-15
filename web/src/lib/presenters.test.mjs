import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import {
  formatBytes,
  formatDockerTimestamp,
  formatDuration,
} from "./presenters.ts";

Deno.test("formats binary byte units", () => {
  assertEquals(formatBytes(0, "en-US"), "0 B");
  assertEquals(formatBytes(1536, "en-US"), "1.5 KiB");
  assertEquals(formatBytes(1536, "de-DE"), "1,5 KiB");
});

Deno.test("formats elapsed time with locale-aware units", () => {
  const twoMinutesAgo = new Date(Date.now() - 120_000).toISOString();
  assertEquals(formatDuration(twoMinutesAgo, "en-US"), "2 minutes");
});

Deno.test("formats docker timestamps as localized relative time", () => {
  const twoMinutesAgo = new Date(Date.now() - 120_000).toISOString();
  assertEquals(formatDockerTimestamp(twoMinutesAgo, "en-US"), "2 minutes ago");
});
