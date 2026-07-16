import { assert, assertEquals, assertRejects } from "jsr:@std/assert@1.0.19";

import {
  LoginRequestBodyTooLargeError,
  loginRequestBodyLimitBytes,
  loginRequestBodySizeStatus,
  readLimitedLoginFormData,
  sanitizeLoginRedirect,
} from "./login.ts";

Deno.test("sanitizes login redirects", () => {
  assertEquals(
    sanitizeLoginRedirect("/tasks?status=running"),
    "/tasks?status=running",
  );
  assertEquals(sanitizeLoginRedirect("//evil.example/path"), "/");
  assertEquals(sanitizeLoginRedirect("https://evil.example/path"), "/");
  assertEquals(sanitizeLoginRedirect(null), "/");
});

Deno.test("rejects oversized login bodies from content-length", () => {
  const request = new Request("https://example.test/login", {
    method: "POST",
    headers: { "content-length": String(loginRequestBodyLimitBytes + 1) },
  });

  assertEquals(loginRequestBodySizeStatus(request), 413);
});

Deno.test("allows login bodies exactly at the limit", async () => {
  const body = new URLSearchParams({
    username: "a".repeat(
      loginRequestBodyLimitBytes - "username=&password=".length,
    ),
    password: "",
  }).toString();
  assertEquals(
    new TextEncoder().encode(body).byteLength,
    loginRequestBodyLimitBytes,
  );

  const formData = await readLimitedLoginFormData(
    new Request("https://example.test/login", {
      method: "POST",
      headers: {
        "content-type": "application/x-www-form-urlencoded",
        "content-length": String(loginRequestBodyLimitBytes),
      },
      body,
    }),
  );

  assertEquals(String(formData.get("password") ?? ""), "");
});

Deno.test("rejects streamed login bodies over the limit", async () => {
  const stream = new ReadableStream({
    start(controller) {
      controller.enqueue(new Uint8Array(loginRequestBodyLimitBytes));
      controller.enqueue(new Uint8Array(1));
      controller.close();
    },
  });

  await assertRejects(
    () =>
      readLimitedLoginFormData(
        new Request("https://example.test/login", {
          method: "POST",
          headers: { "content-type": "application/x-www-form-urlencoded" },
          body: stream,
        }),
      ),
    LoginRequestBodyTooLargeError,
  );
});
