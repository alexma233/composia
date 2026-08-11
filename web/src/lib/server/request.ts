export function isDocumentRequest(request: Request) {
  return (
    request.method === "GET" &&
    (request.headers.get("accept") ?? "").includes("text/html")
  );
}
