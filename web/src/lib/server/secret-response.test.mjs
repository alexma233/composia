import { assertEquals } from "jsr:@std/assert@1.0.19";

import { decryptedSecretResponseInit } from "./secret-response.ts";

Deno.test("marks decrypted secret responses private and non-cacheable", () => {
  assertEquals(decryptedSecretResponseInit(true), {
    headers: { "Cache-Control": "private, no-store" },
  });
  assertEquals(decryptedSecretResponseInit(false), undefined);
});
