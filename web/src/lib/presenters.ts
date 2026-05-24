import type { BadgeVariant } from "$lib/components/ui/badge";

import {
  formatDistanceToNowStrict,
  isAfter,
  isFuture,
  isValid,
  parse,
  parseISO,
  subDays,
} from "date-fns";
import { filesize } from "filesize";

import type { Dictionary } from "$lib/i18n";

export function formatTimestamp(value: string) {
  if (!value) {
    return "-";
  }
  const parsed = parseTimestamp(value);
  return parsed ? parsed.toLocaleString() : value;
}

export function taskStatusTone(status: string): BadgeVariant {
  switch (status) {
    case "running":
      return "outline";
    case "awaiting_confirmation":
      return "outline";
    case "succeeded":
      return "default";
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
      return status || messages.status.unknown;
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
    case "docker_remove":
      return messages.tasks.types.dockerRemove;
    default:
      return type || messages.status.unknown;
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
      return "default";
    case "stopped":
      return "secondary";
    case "error":
      return "destructive";
    default:
      return "outline";
  }
}

export function onlineStatusTone(isOnline: boolean): BadgeVariant {
  return isOnline ? "default" : "secondary";
}

export function containerStateTone(state: string): BadgeVariant {
  const s = (state || "").toLowerCase();
  if (s === "running") return "default";
  if (s === "created" || s === "starting") return "outline";
  if (s === "paused") return "secondary";
  if (s === "restarting" || s === "unhealthy") return "outline";
  if (s === "exited" || s === "dead" || s === "removing") return "destructive";
  return "default";
}

export function isTaskRecent(createdAt: string): boolean {
  const parsed = parseTimestamp(createdAt);
  return parsed ? isAfter(parsed, subDays(new Date(), 1)) : false;
}

export function formatDuration(startedAt: string): string {
  if (!startedAt) return "-";
  const start = parseTimestamp(startedAt);
  return start
    ? formatDistanceToNowStrict(start, { roundingMethod: "floor" })
    : "-";
}

export function formatBytes(bytes: number) {
  return filesize(bytes || 0, { base: 2, round: 2, standard: "jedec" });
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

export function formatDockerTimestamp(timestamp: string) {
  if (!timestamp) {
    return "-";
  }

  const parsed = parseDockerTimestamp(timestamp);
  if (!parsed) {
    return timestamp;
  }

  return isFuture(parsed)
    ? "just now"
    : formatDistanceToNowStrict(parsed, {
        addSuffix: true,
        roundingMethod: "floor",
      });
}

function parseTimestamp(value: string): Date | null {
  const iso = parseISO(value);
  if (isValid(iso)) {
    return iso;
  }

  const parsed = new Date(value);
  return isValid(parsed) ? parsed : null;
}

function parseDockerTimestamp(value: string): Date | null {
  const cleaned = value.replace(/\s+[+-]\d{4}\s+\w+$/, "");
  const parsed = parse(cleaned, "yyyy-MM-dd HH:mm:ss", new Date());
  if (isValid(parsed)) {
    return parsed;
  }

  return parseTimestamp(cleaned);
}
