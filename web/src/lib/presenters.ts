import type { BadgeVariant } from '$lib/components/ui/badge';

import type { Dictionary } from '$lib/i18n';

export function formatTimestamp(value: string) {
  if (!value) {
    return "N/A";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

export function taskStatusTone(status: string): BadgeVariant {
  switch (status) {
    case "running":
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
    case 'running':
      return messages.status.running;
    case 'succeeded':
      return messages.status.succeeded;
    case 'pending':
      return messages.status.pending;
    case 'failed':
      return messages.status.failed;
    case 'cancelled':
      return messages.status.cancelled;
    default:
      return status || messages.status.unknown;
  }
}

export function runtimeStatusLabel(status: string, messages: Dictionary) {
  switch (status) {
    case 'running':
      return messages.status.running;
    case 'stopped':
      return messages.status.stopped;
    case 'pending':
      return messages.status.pending;
    case 'failed':
    case 'error':
      return messages.status.failed;
    case 'succeeded':
      return messages.status.succeeded;
    case 'cancelled':
      return messages.status.cancelled;
    case 'online':
      return messages.status.online;
    case 'offline':
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
    return '-';
  }

  const cleaned = timestamp.replace(/\s+[+-]\d{4}\s+\w+$/, '');
  const parts = cleaned.split(' ');
  const parsed = parts.length === 2 ? new Date(`${parts[0]}T${parts[1]}`) : new Date(cleaned);

  if (Number.isNaN(parsed.getTime())) {
    return timestamp;
  }

  const diff = Math.floor((Date.now() - parsed.getTime()) / 1000);

  if (diff < 0) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}
