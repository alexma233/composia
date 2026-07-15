import { assertEquals } from "jsr:@std/assert@1.0.19";

import {
  decryptedSecretResponseInit,
  setDecryptedSecretResponseHeaders,
} from "./secret-response.ts";

Deno.test("marks decrypted secret responses private and non-cacheable", () => {
  assertEquals(decryptedSecretResponseInit(true), {
    headers: { "Cache-Control": "private, no-store" },
  });
  assertEquals(decryptedSecretResponseInit(false), undefined);
});

Deno.test(
  "sets decrypted secret page-load headers only for encrypted content",
  () => {
    const headers = [];
    setDecryptedSecretResponseHeaders(
      (nextHeaders) => headers.push(nextHeaders),
      false,
    );
    setDecryptedSecretResponseHeaders(
      (nextHeaders) => headers.push(nextHeaders),
      true,
    );

    assertEquals(headers, [{ "Cache-Control": "private, no-store" }]);
  },
);
