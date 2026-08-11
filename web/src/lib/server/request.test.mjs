import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { isDocumentRequest } from "./request.ts";

Deno.test("document requests require GET and an HTML accept header", () => {
  assertEquals(
    isDocumentRequest(
      new Request("https://example.test", { headers: { Accept: "text/html" } }),
    ),
    true,
  );
  assertEquals(isDocumentRequest(new Request("https://example.test")), false);
  assertEquals(
    isDocumentRequest(
      new Request("https://example.test", {
        method: "POST",
        headers: { Accept: "text/html" },
      }),
    ),
    false,
  );
});
