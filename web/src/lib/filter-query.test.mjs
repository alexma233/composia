import { assertEquals } from "jsr:@std/assert@1.0.19/equals";

import { filterQuerySignature } from "./filter-query.ts";

Deno.test("filter query signatures change only when URL filters change", () => {
  assertEquals(
    filterQuerySignature(1, { status: ["running"], nodeId: ["n1"] }),
    "status=running&nodeId=n1",
  );
  assertEquals(
    filterQuerySignature(2, { status: ["running"], nodeId: ["n1"] }),
    "page=2&status=running&nodeId=n1",
  );
});
