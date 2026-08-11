const minimumSize = 1024;
const gzipEtagSuffix = "-gzip";

const compressibleContentTypes = [
  "application/javascript",
  "application/json",
  "application/manifest+json",
  "application/wasm",
  "application/xml",
  "image/svg+xml",
];

function acceptsGzip(value) {
  const encodings = new Map(
    value.split(",").map((entry) => {
      const [encoding, ...parameters] = entry.trim().toLowerCase().split(";");
      const quality = parameters
        .map((parameter) => parameter.trim().match(/^q=(\d*(?:\.\d+)?)$/)?.[1])
        .find(Boolean);
      return [encoding, quality === undefined ? 1 : Number(quality)];
    }),
  );
  return (encodings.get("gzip") ?? encodings.get("*") ?? 0) > 0;
}

function appendVary(headers, value) {
  const values = (headers.get("vary") ?? "")
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
  if (!values.some((entry) => entry.toLowerCase() === value.toLowerCase())) {
    values.push(value);
  }
  headers.set("vary", values.join(", "));
}

function gzipEtag(value) {
  return value.replace(
    /^(W\/)?"([^\"]*)"$/,
    (_, weak = "", opaque) => `${weak}"${opaque}${gzipEtagSuffix}"`,
  );
}

function identityEtags(value) {
  if (value.trim() === "*") return "*";
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry.endsWith(`${gzipEtagSuffix}"`))
    .map((entry) => entry.replace(new RegExp(`${gzipEtagSuffix}("$)`), "$1"))
    .join(", ");
}

export function prepareCompressionRequest(request) {
  if (
    request.method !== "GET" ||
    !acceptsGzip(request.headers.get("accept-encoding") ?? "")
  ) {
    return request;
  }

  const ifNoneMatch = request.headers.get("if-none-match");
  if (!ifNoneMatch) return request;

  const headers = new Headers(request.headers);
  const etags = identityEtags(ifNoneMatch);
  if (etags) headers.set("if-none-match", etags);
  else headers.delete("if-none-match");
  return new Request(request, { headers });
}

function isCompressible(response) {
  const contentType = response.headers.get("content-type")?.toLowerCase() ?? "";
  const contentLengthHeader = response.headers.get("content-length");
  const contentLength = contentLengthHeader ? Number(contentLengthHeader) : NaN;
  return (
    response.status !== 204 &&
    response.status !== 205 &&
    response.status !== 304 &&
    !response.headers.has("content-encoding") &&
    !response.headers.has("content-range") &&
    !response.headers
      .get("cache-control")
      ?.toLowerCase()
      .includes("no-transform") &&
    !contentType.startsWith("text/event-stream") &&
    (!Number.isFinite(contentLength) || contentLength >= minimumSize) &&
    (contentType.startsWith("text/") ||
      compressibleContentTypes.some((type) => contentType.startsWith(type)))
  );
}

export function compressResponse(request, response) {
  if (response.status === 304) {
    const headers = new Headers(response.headers);
    appendVary(headers, "Accept-Encoding");
    if (
      acceptsGzip(request.headers.get("accept-encoding") ?? "") &&
      request.headers
        .get("if-none-match")
        ?.split(",")
        .some(
          (etag) =>
            etag.trim() === "*" || etag.trim().endsWith(`${gzipEtagSuffix}"`),
        )
    ) {
      headers.set("content-encoding", "gzip");
      const etag = headers.get("etag");
      if (etag) headers.set("etag", gzipEtag(etag));
    }
    return new Response(null, {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  }

  if (!isCompressible(response)) {
    return response;
  }

  const headers = new Headers(response.headers);
  appendVary(headers, "Accept-Encoding");

  if (
    request.method === "HEAD" ||
    !acceptsGzip(request.headers.get("accept-encoding") ?? "")
  ) {
    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  }

  headers.set("content-encoding", "gzip");
  headers.delete("content-length");
  const etag = headers.get("etag");
  if (etag) headers.set("etag", gzipEtag(etag));

  return new Response(
    response.body?.pipeThrough(new CompressionStream("gzip")) ?? null,
    {
      status: response.status,
      statusText: response.statusText,
      headers,
    },
  );
}
