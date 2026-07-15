import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { enUS } from "./i18n/messages/en-us.ts";
import {
  formatBytes,
  formatDockerTimestamp,
  formatDuration,
  taskStatusLabel,
  taskStepNameLabel,
  taskTypeLabel,
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

Deno.test("formats task enums through i18n labels", () => {
  assertEquals(
    taskTypeLabel("cloudflare_tunnel_sync", enUS),
    "Cloudflare Tunnel sync",
  );
  assertEquals(taskStepNameLabel("compose_down", enUS), "Compose down");
  assertEquals(taskStatusLabel("TASK_STATUS_FAILED", enUS), "Unknown");
});
