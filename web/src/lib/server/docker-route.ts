export function svelteKitRouteParam(value: string) {
  return value;
}

export function dockerConnectHeaders(
  token: string,
  controllerHeaders: Record<string, string>,
) {
  return {
    ...controllerHeaders,
    Authorization: `Bearer ${token}`,
    "Connect-Protocol-Version": "1",
    "Content-Type": "application/connect+json",
    "X-Composia-Source": "web",
  };
}
