export const controllerRpcTimeoutMs = 30_000;

type RpcRequest = Record<string, unknown>;

export class ControllerRpcError extends Error {
  readonly status: number;
  readonly code: string | null;
  readonly procedure: string;

  constructor(options: {
    message: string;
    status: number;
    code?: string | null;
    procedure: string;
  }) {
    super(options.message);
    this.name = "ControllerRpcError";
    this.status = options.status;
    this.code = options.code ?? null;
    this.procedure = options.procedure;
  }
}

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

export async function controllerRpcCall<T>(options: {
  baseUrl: string;
  token: string;
  procedure: string;
  body: RpcRequest;
  controllerHeaders: Record<string, string>;
  extraHeaders?: Record<string, string>;
  requestSignal?: AbortSignal;
  timeoutMs?: number;
}): Promise<T> {
  const deadline = controllerRpcDeadline(
    options.requestSignal,
    options.timeoutMs,
  );

  try {
    const response = await fetch(`${options.baseUrl}${options.procedure}`, {
      method: "POST",
      headers: controllerRpcHeaders(
        options.token,
        options.controllerHeaders,
        options.extraHeaders,
      ),
      body: JSON.stringify(options.body),
      signal: deadline.signal,
    });

    if (!response.ok) {
      throw await controllerRpcErrorFromResponse(response, options.procedure);
    }

    return (await response.json()) as T;
  } catch (error) {
    if (deadline.timedOut()) {
      throw new ControllerRpcError({
        message: `Controller RPC ${options.procedure} timed out.`,
        status: 504,
        code: "DEADLINE_EXCEEDED",
        procedure: options.procedure,
      });
    }
    throw error;
  } finally {
    deadline.cancel();
  }
}

async function controllerRpcErrorFromResponse(
  response: Response,
  procedure: string,
): Promise<ControllerRpcError> {
  const text = await response.text();
  const fallbackMessage = `Controller RPC ${procedure} failed.`;
  let message = fallbackMessage;
  let code: string | null = null;

  if (text) {
    try {
      const parsed = JSON.parse(text) as {
        code?: unknown;
        message?: unknown;
        error?: unknown;
      };
      if (typeof parsed.code === "string") {
        code = parsed.code;
      }
      if (typeof parsed.message === "string" && parsed.message.trim()) {
        message = parsed.message;
      } else if (typeof parsed.error === "string" && parsed.error.trim()) {
        message = parsed.error;
      } else {
        message = text;
      }
    } catch {
      message = text;
    }
  }

  return new ControllerRpcError({
    message,
    status: response.status,
    code,
    procedure,
  });
}
