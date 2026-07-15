export const controllerRpcTimeoutMs = 30_000;

export function controllerRpcHeaders(
  token: string,
  controllerHeaders: Record<string, string>,
  extraHeaders: Record<string, string> = {},
) {
  return {
    ...controllerHeaders,
    Authorization: `Bearer ${token}`,
    "Connect-Protocol-Version": "1",
    "Connect-Timeout-Ms": String(controllerRpcTimeoutMs),
    "Content-Type": "application/json",
    "X-Composia-Source": "web",
    ...extraHeaders,
  };
}

export function controllerRpcDeadline(
  requestSignal: AbortSignal | undefined,
  timeoutMs = controllerRpcTimeoutMs,
) {
  const controller = new AbortController();
  let timedOut = false;

  const abortFromRequest = () => controller.abort(requestSignal?.reason);
  if (requestSignal?.aborted) {
    abortFromRequest();
  } else {
    requestSignal?.addEventListener("abort", abortFromRequest, { once: true });
  }

  const timeout = setTimeout(() => {
    timedOut = true;
    controller.abort(
      new DOMException("Controller RPC timed out.", "TimeoutError"),
    );
  }, timeoutMs);

  return {
    signal: controller.signal,
    timedOut: () => timedOut,
    cancel: () => {
      clearTimeout(timeout);
      requestSignal?.removeEventListener("abort", abortFromRequest);
    },
  };
}
