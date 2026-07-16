import { assert } from "jsr:@std/assert@1.0.19/assert";
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
  assertEquals(taskTypeLabel("image_check", enUS), "Image check");
  assertEquals(taskTypeLabel("rustic_init", enUS), "Rustic init");
  assertEquals(taskStepNameLabel("compose_down", enUS), "Compose down");
  assertEquals(taskStatusLabel("TASK_STATUS_FAILED", enUS), "Unknown");
});

Deno.test("maps concrete backend task types", () => {
  const backendTaskTypes = [
    "deploy",
    "stop",
    "restart",
    "update",
    "backup",
    "restore",
    "migrate",
    "dns_update",
    "caddy_sync",
    "caddy_reload",
    "image_check",
    "prune",
    "rustic_init",
    "rustic_forget",
    "rustic_prune",
    "docker_start",
    "docker_stop",
    "docker_restart",
    "migrate_rollback",
    "docker_remove_container",
    "docker_remove_network",
    "docker_remove_volume",
    "docker_remove_image",
    "cloudflare_tunnel_sync",
  ];

  for (const taskType of backendTaskTypes) {
    assert(taskTypeLabel(taskType, enUS) !== enUS.status.unknown, taskType);
  }
});
