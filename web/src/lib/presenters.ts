import type { BadgeVariant } from "$lib/components/ui/badge";

import type { Dictionary } from "$lib/i18n";

export function formatTimestamp(value: string, locale = currentLocale()) {
  if (!value) {
    return "-";
  }
  const parsed = parseTimestamp(value);
  return parsed
    ? new Intl.DateTimeFormat(locale, dateTimeFormat).format(parsed)
    : value;
}

export function taskStatusTone(status: string): BadgeVariant {
  switch (status) {
    case "running":
      return "info";
    case "awaiting_confirmation":
      return "warning";
    case "succeeded":
      return "success";
    case "pending":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "secondary";
  }
}

export function taskStatusLabel(status: string, messages: Dictionary) {
  switch (status) {
    case "running":
      return messages.status.running;
    case "succeeded":
      return messages.status.succeeded;
    case "pending":
      return messages.status.pending;
    case "awaiting_confirmation":
      return messages.status.awaitingConfirmation;
    case "failed":
      return messages.status.failed;
    case "cancelled":
      return messages.status.cancelled;
    default:
      return messages.status.unknown;
  }
}

export function taskTypeLabel(type: string, messages: Dictionary) {
  switch (type) {
    case "deploy":
      return messages.tasks.types.deploy;
    case "update":
      return messages.tasks.types.update;
    case "restart":
      return messages.tasks.types.restart;
    case "stop":
      return messages.tasks.types.stop;
    case "backup":
      return messages.tasks.types.backup;
    case "restore":
      return messages.tasks.types.restore;
    case "migrate":
      return messages.tasks.types.migrate;
    case "migrate_rollback":
      return messages.tasks.types.migrateRollback;
    case "dns_update":
      return messages.tasks.types.dnsUpdate;
    case "caddy_sync":
      return messages.tasks.types.caddySync;
    case "cloudflare_tunnel_sync":
      return messages.tasks.types.cloudflareTunnelSync;
    case "caddy_reload":
      return messages.tasks.types.caddyReload;
    case "prune":
      return messages.tasks.types.prune;
    case "rustic_forget":
      return messages.tasks.types.rusticForget;
    case "rustic_prune":
      return messages.tasks.types.rusticPrune;
    case "docker_start":
      return messages.tasks.types.dockerStart;
    case "docker_stop":
      return messages.tasks.types.dockerStop;
    case "docker_restart":
      return messages.tasks.types.dockerRestart;
    case "docker_remove_container":
      return messages.tasks.types.dockerRemoveContainer;
    case "docker_remove_network":
      return messages.tasks.types.dockerRemoveNetwork;
    case "docker_remove_volume":
      return messages.tasks.types.dockerRemoveVolume;
    case "docker_remove_image":
      return messages.tasks.types.dockerRemoveImage;
    case "unspecified":
      return messages.status.unknown;
    default:
      return messages.status.unknown;
  }
}

export function taskStepNameLabel(stepName: string, messages: Dictionary) {
  switch (stepName) {
    case "render":
      return messages.tasks.steps.render;
    case "pull":
      return messages.tasks.steps.pull;
    case "backup":
      return messages.tasks.steps.backup;
    case "compose_down":
      return messages.tasks.steps.composeDown;
    case "compose_up":
      return messages.tasks.steps.composeUp;
    case "transfer":
      return messages.tasks.steps.transfer;
    case "restore":
      return messages.tasks.steps.restore;
    case "dns_update":
      return messages.tasks.steps.dnsUpdate;
    case "caddy_sync":
      return messages.tasks.steps.caddySync;
    case "caddy_reload":
      return messages.tasks.steps.caddyReload;
    case "image_check":
      return messages.tasks.steps.imageCheck;
    case "init":
      return messages.tasks.steps.init;
    case "prune":
      return messages.tasks.steps.prune;
    case "awaiting_confirmation":
      return messages.tasks.steps.awaitingConfirmation;
    case "persist_repo":
      return messages.tasks.steps.persistRepo;
    case "finalize":
      return messages.tasks.steps.finalize;
    case "docker_start":
      return messages.tasks.steps.dockerStart;
    case "docker_stop":
      return messages.tasks.steps.dockerStop;
    case "docker_restart":
      return messages.tasks.steps.dockerRestart;
    case "docker_remove":
      return messages.tasks.steps.dockerRemove;
    case "cloudflare_tunnel_sync":
      return messages.tasks.steps.cloudflareTunnelSync;
    case "unspecified":
      return messages.tasks.steps.unspecified;
    default:
      return messages.tasks.steps.unspecified;
  }
}

export function runtimeStatusLabel(status: string, messages: Dictionary) {
  switch (status) {
    case "running":
      return messages.status.running;
    case "stopped":
      return messages.status.stopped;
    case "pending":
      return messages.status.pending;
    case "failed":
    case "error":
      return messages.status.failed;
    case "succeeded":
      return messages.status.succeeded;
    case "cancelled":
      return messages.status.cancelled;
    case "online":
      return messages.status.online;
    case "offline":
      return messages.status.offline;
    default:
      return status || messages.status.unknown;
  }
}

export function runtimeStatusTone(status: string): BadgeVariant {
  switch (status) {
    case "running":
      return "info";
    case "succeeded":
      return "success";
    case "stopped":
      return "secondary";
    case "failed":
    case "error":
      return "destructive";
    default:
      return "outline";
  }
}

export function onlineStatusTone(isOnline: boolean): BadgeVariant {
  return isOnline ? "success" : "secondary";
}

export function containerStateTone(state: string): BadgeVariant {
  const s = (state || "").toLowerCase();
  if (s === "running") return "success";
  if (s === "created" || s === "starting") return "info";
  if (s === "paused") return "secondary";
  if (s === "restarting" || s === "unhealthy") return "warning";
  if (s === "exited" || s === "dead" || s === "removing") return "destructive";
  return "secondary";
}

export function isTaskRecent(createdAt: string): boolean {
  const parsed = parseTimestamp(createdAt);
  return parsed ? parsed.getTime() > Date.now() - 86_400_000 : false;
}

export function formatDuration(
  startedAt: string,
  locale = currentLocale(),
): string {
  if (!startedAt) return "-";
  const start = parseTimestamp(startedAt);
  return start ? formatDistance(start, new Date(), locale) : "-";
}

export function formatBytes(bytes: number, locale = currentLocale()) {
  const value = Math.max(0, bytes || 0);
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  const unitIndex = Math.min(
    Math.floor(Math.log(Math.max(value, 1)) / Math.log(1024)),
    units.length - 1,
  );
  const amount = value / 1024 ** unitIndex;
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(amount)} ${units[unitIndex]}`;
}

export function formatShortId(value: string, length = 12) {
  if (!value) {
    return "-";
  }

  return value.length > length ? value.slice(0, length) : value;
}

export function parseJsonList(rawJson: string) {
  if (!rawJson) {
    return null;
  }

  const parsed = JSON.parse(rawJson);
  return Array.isArray(parsed) ? parsed[0] : parsed;
}

export function formatDockerTimestamp(
  timestamp: string,
  locale = currentLocale(),
) {
  if (!timestamp) {
    return "-";
  }

  const parsed = parseDockerTimestamp(timestamp);
  if (!parsed) {
    return timestamp;
  }

  return formatRelativeTime(parsed, new Date(), locale);
}

const dateTimeFormat: Intl.DateTimeFormatOptions = {
  dateStyle: "medium",
  timeStyle: "medium",
};

function currentLocale() {
  if (typeof document !== "undefined" && document.documentElement.lang) {
    return document.documentElement.lang;
  }
  if (typeof navigator !== "undefined") {
    return navigator.languages?.[0] ?? navigator.language;
  }
  return "en-US";
}

function parseTimestamp(value: string): Date | null {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function parseDockerTimestamp(value: string): Date | null {
  const cleaned = value.replace(/\s+[+-]\d{4}\s+\w+$/, "");
  return parseTimestamp(cleaned.replace(" ", "T"));
}

function formatDistance(from: Date, to: Date, locale: string) {
  const [amount, unit] = elapsedUnit(from, to);
  return new Intl.NumberFormat(locale, {
    style: "unit",
    unit,
    unitDisplay: "long",
    maximumFractionDigits: 0,
  }).format(amount);
}

function formatRelativeTime(from: Date, to: Date, locale: string) {
  const [amount, unit] = elapsedUnit(from, to);
  const sign = from.getTime() > to.getTime() ? amount : -amount;
  return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(
    sign,
    unit,
  );
}

function elapsedUnit(
  from: Date,
  to: Date,
): [number, Intl.RelativeTimeFormatUnit] {
  const seconds = Math.max(
    0,
    Math.floor(Math.abs(to.getTime() - from.getTime()) / 1000),
  );
  const units: Array<[number, Intl.RelativeTimeFormatUnit]> = [
    [31_536_000, "year"],
    [2_592_000, "month"],
    [604_800, "week"],
    [86_400, "day"],
    [3_600, "hour"],
    [60, "minute"],
    [1, "second"],
  ];
  const [size, unit] = units.find(([size]) => seconds >= size) ?? units.at(-1)!;
  return [Math.floor(seconds / size), unit];
}
