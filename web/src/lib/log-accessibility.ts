export function logLiveMode(
  state: string,
  hasAnnouncements: boolean,
): "off" | "polite" {
  if (state === "streaming" || hasAnnouncements) {
    return "polite";
  }
  return "off";
}
