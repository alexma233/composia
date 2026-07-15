export const loginRequestBodyLimitBytes = 16 * 1024;

export class LoginRequestBodyTooLargeError extends Error {
  constructor() {
    super("Login request body is too large.");
    this.name = "LoginRequestBodyTooLargeError";
  }
}

export function loginRequestBodySizeStatus(request: Request) {
  if (request.method !== "POST" || new URL(request.url).pathname !== "/login") {
    return null;
  }

  const contentLength = request.headers.get("content-length");
  if (!contentLength) {
    return null;
  }

  const parsed = Number(contentLength);
  return Number.isFinite(parsed) && parsed > loginRequestBodyLimitBytes
    ? 413
    : null;
}

export async function readLimitedLoginFormData(request: Request) {
  const body = await readLimitedRequestBody(
    request,
    loginRequestBodyLimitBytes,
  );
  return new Request(request.url, {
    method: request.method,
    headers: request.headers,
    body,
  }).formData();
}

export function sanitizeLoginRedirect(value: string | null) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}

async function readLimitedRequestBody(request: Request, limitBytes: number) {
  const contentLength = request.headers.get("content-length");
  if (contentLength) {
    const parsed = Number(contentLength);
    if (Number.isFinite(parsed) && parsed > limitBytes) {
      throw new LoginRequestBodyTooLargeError();
    }
  }

  if (!request.body) {
    return new Uint8Array();
  }

  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      if (!value) {
        continue;
      }
      size += value.byteLength;
      if (size > limitBytes) {
        throw new LoginRequestBodyTooLargeError();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}
