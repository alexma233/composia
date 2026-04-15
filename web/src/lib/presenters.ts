import type { BadgeVariant } from "$lib/components/ui/badge";

import type { Dictionary } from "$lib/i18n";

export function formatTimestamp(value: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
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
    case "docker_list":
      return messages.tasks.types.dockerList;
    case "docker_inspect":
      return messages.tasks.types.dockerInspect;
    case "docker_start":
      return messages.tasks.types.dockerStart;
    case "docker_stop":
      return messages.tasks.types.dockerStop;
    case "docker_restart":
      return messages.tasks.types.dockerRestart;
    case "docker_logs":
      return messages.tasks.types.dockerLogs;
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

export function formatBytes(bytes: number) {
  if (bytes === 0 || !bytes) {
    return "0 B";
  }

  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
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

  const cleaned = timestamp.replace(/\s+[+-]\d{4}\s+\w+$/, "");
  const parts = cleaned.split(" ");
  const parsed =
    parts.length === 2
      ? new Date(`${parts[0]}T${parts[1]}`)
      : new Date(cleaned);

  if (Number.isNaN(parsed.getTime())) {
    return timestamp;
  }

  const diff = Math.floor((Date.now() - parsed.getTime()) / 1000);

  if (diff < 0) return "just now";
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}
