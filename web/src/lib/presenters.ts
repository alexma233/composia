export function formatTimestamp(value: string) {
  if (!value) {
    return "N/A";
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

export function taskStatusTone(status: string) {
  switch (status) {
    case "running":
      return "info";
    case "succeeded":
      return "success";
    case "pending":
      return "warning";
    case "failed":
      return "danger";
    default:
      return "secondary";
  }
}

export function runtimeStatusTone(status: string) {
  switch (status) {
    case "running":
      return "success";
    case "stopped":
      return "secondary";
    case "error":
      return "danger";
    default:
      return "warning";
  }
}

export function onlineStatusTone(isOnline: boolean) {
  return isOnline ? "success" : "secondary";
}
