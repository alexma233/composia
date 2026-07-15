import { assert, assertRejects } from "jsr:@std/assert@1.0.19";

import { verifyArgon2 } from "./argon2.ts";

const adminHash =
  "$argon2id$v=19$m=65536,t=3,p=4$/wh05hbH5ipiT42CK+GxVA$2unNmHbsRe5ZkFgIkHNekBGk6KH+79sZAPB9qmRrUlQ";

Deno.test("verifies an existing Argon2 PHC hash", async () => {
  assert(await verifyArgon2(adminHash, "admin"));
});

Deno.test("rejects the wrong Argon2 password", async () => {
  assert(!(await verifyArgon2(adminHash, "wrong-password")));
});

Deno.test("rejects malformed Argon2 hashes", async () => {
  await assertRejects(() => verifyArgon2("not-a-phc-hash", "admin"));
});
